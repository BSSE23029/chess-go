// Package lan provides dependency-free DNS-SD/mDNS advertisement and
// discovery for chess hosts on a local network.
package lan

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// ServiceType is the DNS-SD service advertised by chess hosts.
const ServiceType = "_chess._tcp.local."

const (
	mdnsPort  = 5353
	maxPacket = 9000
)

var mdnsGroup = net.IPv4(224, 0, 0, 251)

// Service describes a discovered or advertised chess host.
type Service struct {
	Instance string            `json:"instance"`
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Advertiser answers DNS-SD queries and periodically announces one service.
type Advertiser struct {
	Service  Service
	Interval time.Duration
	mu       sync.Mutex
	conn     *net.UDPConn
}

// NewAdvertiser validates and normalizes a service descriptor.
func NewAdvertiser(service Service) (*Advertiser, error) {
	if strings.TrimSpace(service.Instance) == "" || service.Port < 1 || service.Port > 65535 {
		return nil, errors.New("LAN service requires instance and valid port")
	}
	if strings.TrimSpace(service.Host) == "" {
		host, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		service.Host = host
	}
	service.Instance = instanceName(service.Instance)
	if service.Metadata != nil {
		copy := make(map[string]string, len(service.Metadata))
		for key, value := range service.Metadata {
			if strings.TrimSpace(key) != "" {
				copy[key] = value
			}
		}
		service.Metadata = copy
	}
	return &Advertiser{Service: service, Interval: 5 * time.Second}, nil
}

// Serve runs the mDNS responder until ctx is canceled.
func (a *Advertiser) Serve(ctx context.Context) error {
	if a == nil {
		return errors.New("nil LAN advertiser")
	}
	conn, err := net.ListenMulticastUDP("udp4", nil, &net.UDPAddr{IP: mdnsGroup, Port: mdnsPort})
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.conn = conn
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.conn = nil
		a.mu.Unlock()
		_ = conn.Close()
	}()
	interval := a.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	group := &net.UDPAddr{IP: mdnsGroup, Port: mdnsPort}
	_ = a.send(conn, group)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(interval)); err != nil {
			return err
		}
		buffer := make([]byte, maxPacket)
		n, sender, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			if timeoutError(err) {
				if err := a.send(conn, group); err != nil {
					return err
				}
				select {
				case <-ctx.Done():
					return nil
				default:
					continue
				}
			}
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
		if packetIsQuery(buffer[:n]) {
			if _, err := conn.WriteToUDP(responsePacket(a.Service), sender); err != nil {
				return err
			}
		}
	}
}

// Close stops an active advertiser.
func (a *Advertiser) Close() error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn == nil {
		return nil
	}
	return a.conn.Close()
}

// Discover sends an mDNS query and collects services until ctx ends. Without a
// deadline it waits two seconds, which is long enough for LAN announcements.
func Discover(ctx context.Context) ([]Service, error) {
	if ctx == nil {
		return nil, errors.New("nil discovery context")
	}
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if _, err := conn.WriteToUDP(queryPacket(), &net.UDPAddr{IP: mdnsGroup, Port: mdnsPort}); err != nil {
		return nil, err
	}
	services := make(map[string]Service)
	for {
		if err := conn.SetReadDeadline(deadline); err != nil {
			return nil, err
		}
		buffer := make([]byte, maxPacket)
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if timeoutError(err) || errors.Is(err, net.ErrClosed) {
				break
			}
			return nil, err
		}
		for _, service := range servicesFromPacket(buffer[:n]) {
			services[service.Instance] = service
		}
		select {
		case <-ctx.Done():
			break
		default:
			continue
		}
		break
	}
	result := make([]Service, 0, len(services))
	for _, service := range services {
		result = append(result, service)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Instance < result[j].Instance })
	return result, nil
}

func (a *Advertiser) send(conn *net.UDPConn, destination *net.UDPAddr) error {
	_, err := conn.WriteToUDP(responsePacket(a.Service), destination)
	return err
}

func instanceName(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, ".") {
		value = strings.TrimSuffix(value, ".")
	}
	if strings.HasSuffix(strings.ToLower(value), strings.TrimSuffix(ServiceType, ".")) {
		return value + "."
	}
	return value + "." + ServiceType
}

func queryPacket() []byte {
	packet := make([]byte, 12, 64)
	binary.BigEndian.PutUint16(packet[4:6], 1)
	packet = append(packet, encodeName(ServiceType)...)
	packet = appendUint16(packet, 12)
	packet = appendUint16(packet, 1)
	return packet
}

func responsePacket(service Service) []byte {
	packet := make([]byte, 12)
	binary.BigEndian.PutUint16(packet[2:4], 0x8400)
	instance := instanceName(service.Instance)
	host := strings.TrimSuffix(service.Host, ".")
	if strings.HasSuffix(strings.ToLower(host), ".local") {
		host += "."
	} else {
		host += ".local."
	}
	answerCount := uint16(3)
	if ip := net.ParseIP(strings.TrimSuffix(service.Host, ".")); ip != nil && ip.To4() != nil {
		answerCount++
	}
	binary.BigEndian.PutUint16(packet[6:8], answerCount)
	packet = appendRR(packet, ServiceType, 12, 120, encodeName(instance))
	srv := make([]byte, 6)
	binary.BigEndian.PutUint16(srv[4:6], uint16(service.Port))
	srv = append(srv, encodeName(host)...)
	packet = appendRR(packet, instance, 33, 120, srv)
	txt := make([]byte, 0)
	keys := make([]string, 0, len(service.Metadata))
	for key := range service.Metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := key + "=" + service.Metadata[key]
		if len(value) > 255 {
			value = value[:255]
		}
		txt = append(txt, byte(len(value)))
		txt = append(txt, value...)
	}
	if len(txt) == 0 {
		txt = []byte{0}
	}
	packet = appendRR(packet, instance, 16, 120, txt)
	if ip := net.ParseIP(strings.TrimSuffix(service.Host, ".")); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			packet = appendRR(packet, host, 1, 120, ipv4)
		}
	}
	return packet
}

func appendRR(packet []byte, name string, recordType uint16, ttl uint32, data []byte) []byte {
	packet = append(packet, encodeName(name)...)
	packet = appendUint16(packet, recordType)
	packet = appendUint16(packet, 1)
	ttlBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(ttlBytes, ttl)
	packet = append(packet, ttlBytes...)
	packet = appendUint16(packet, uint16(len(data)))
	return append(packet, data...)
}

func appendUint16(packet []byte, value uint16) []byte {
	data := []byte{0, 0}
	binary.BigEndian.PutUint16(data, value)
	return append(packet, data...)
}

func encodeName(name string) []byte {
	name = strings.TrimSuffix(name, ".")
	encoded := make([]byte, 0, len(name)+2)
	for _, label := range strings.Split(name, ".") {
		if len(label) > 63 {
			label = label[:63]
		}
		encoded = append(encoded, byte(len(label)))
		encoded = append(encoded, label...)
	}
	return append(encoded, 0)
}

func packetIsQuery(packet []byte) bool {
	return len(packet) >= 12 && binary.BigEndian.Uint16(packet[2:4])&0x8000 == 0 && binary.BigEndian.Uint16(packet[4:6]) > 0
}

func servicesFromPacket(packet []byte) []Service {
	if len(packet) < 12 || binary.BigEndian.Uint16(packet[2:4])&0x8000 == 0 {
		return nil
	}
	index := 12
	questions := int(binary.BigEndian.Uint16(packet[4:6]))
	answers := int(binary.BigEndian.Uint16(packet[6:8])) + int(binary.BigEndian.Uint16(packet[8:10])) + int(binary.BigEndian.Uint16(packet[10:12]))
	for range questions {
		if _, next, err := readName(packet, index); err != nil {
			return nil
		} else {
			index = next + 4
		}
	}
	instances := make(map[string]Service)
	for range answers {
		name, next, err := readName(packet, index)
		if err != nil || next+10 > len(packet) {
			return nil
		}
		typeID := binary.BigEndian.Uint16(packet[next : next+2])
		length := int(binary.BigEndian.Uint16(packet[next+8 : next+10]))
		dataStart := next + 10
		if dataStart+length > len(packet) {
			return nil
		}
		data := packet[dataStart : dataStart+length]
		index = dataStart + length
		name = strings.TrimSuffix(name, ".") + "."
		switch typeID {
		case 12:
			instance, _, err := readName(packet, dataStart)
			if err == nil && strings.HasSuffix(strings.ToLower(instance), strings.TrimSuffix(ServiceType, ".")) {
				instances[instance] = Service{Instance: instance}
			}
		case 33:
			if len(data) < 6 {
				continue
			}
			instance := instances[name]
			instance.Instance = name
			instance.Port = int(binary.BigEndian.Uint16(data[4:6]))
			host, _, err := readName(packet, dataStart+6)
			if err == nil {
				instance.Host = strings.TrimSuffix(host, ".")
			}
			instances[name] = instance
		case 16:
			instance := instances[name]
			instance.Instance = name
			for offset := 0; offset < len(data); {
				length := int(data[offset])
				offset++
				if offset+length > len(data) {
					break
				}
				key, value, _ := strings.Cut(string(data[offset:offset+length]), "=")
				if instance.Metadata == nil {
					instance.Metadata = make(map[string]string)
				}
				instance.Metadata[key] = value
				offset += length
			}
			instances[name] = instance
		case 1:
			if len(data) == 4 {
				instanceHost := net.IP(data).String()
				for key, instance := range instances {
					if strings.EqualFold(instance.Host, strings.TrimSuffix(name, ".")) {
						instance.Host = instanceHost
						instances[key] = instance
					}
				}
			}
		}
	}
	result := make([]Service, 0, len(instances))
	for _, service := range instances {
		if service.Port > 0 {
			result = append(result, service)
		}
	}
	return result
}

func readName(packet []byte, offset int) (string, int, error) {
	return readNameDepth(packet, offset, 0)
}

func readNameDepth(packet []byte, offset, depth int) (string, int, error) {
	if depth > 16 || offset >= len(packet) {
		return "", offset, errors.New("invalid DNS name")
	}
	labels := make([]string, 0, 4)
	start := offset
	for {
		if offset >= len(packet) {
			return "", offset, errors.New("truncated DNS name")
		}
		length := int(packet[offset])
		offset++
		if length == 0 {
			return strings.Join(labels, ".") + ".", offset, nil
		}
		if length&0xc0 == 0xc0 {
			if offset >= len(packet) {
				return "", offset, errors.New("truncated DNS pointer")
			}
			pointer := (length&0x3f)<<8 | int(packet[offset])
			name, _, err := readNameDepth(packet, pointer, depth+1)
			if err != nil {
				return "", offset + 1, err
			}
			if len(labels) == 0 {
				return name, offset + 1, nil
			}
			return strings.Join(append(labels, strings.TrimSuffix(name, ".")), ".") + ".", offset + 1, nil
		}
		if length > 63 || offset+length > len(packet) {
			return "", offset, errors.New("invalid DNS label")
		}
		labels = append(labels, string(packet[offset:offset+length]))
		offset += length
		if offset == start {
			return "", offset, errors.New("DNS name did not advance")
		}
	}
}

func timeoutError(err error) bool {
	if netError, ok := err.(net.Error); ok {
		return netError.Timeout()
	}
	return false
}
