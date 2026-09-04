package lan

import (
	"encoding/binary"
	"testing"
)

func TestDNSServicePacketRoundTrip(t *testing.T) {
	advertiser, err := NewAdvertiser(Service{Instance: "board", Host: "host.local", Port: 8080, Metadata: map[string]string{"version": "1", "mode": "lan"}})
	if err != nil {
		t.Fatal(err)
	}
	packet := responsePacket(advertiser.Service)
	if binary.BigEndian.Uint16(packet[2:4]) != 0x8400 {
		t.Fatalf("response flags = %#x", binary.BigEndian.Uint16(packet[2:4]))
	}
	services := servicesFromPacket(packet)
	if len(services) != 1 || services[0].Instance != "board._chess._tcp.local." || services[0].Host != "host.local" || services[0].Port != 8080 {
		t.Fatalf("decoded services = %#v", services)
	}
	if services[0].Metadata["version"] != "1" || services[0].Metadata["mode"] != "lan" {
		t.Fatalf("decoded metadata = %#v", services[0].Metadata)
	}
	query := queryPacket()
	if !packetIsQuery(query) || binary.BigEndian.Uint16(query[4:6]) != 1 {
		t.Fatalf("query packet invalid")
	}
}

func TestDNSServiceValidationAndMalformedPackets(t *testing.T) {
	for _, service := range []Service{{}, {Instance: "x", Port: 0}, {Instance: "x", Port: 65536}} {
		if _, err := NewAdvertiser(service); err == nil {
			t.Fatalf("invalid service accepted: %#v", service)
		}
	}
	advertiser, err := NewAdvertiser(Service{Instance: "x", Host: "host", Port: 1, Metadata: map[string]string{"ok": "yes"}})
	if err != nil {
		t.Fatal(err)
	}
	advertiser.Service.Metadata["ok"] = "changed"
	if servicesFromPacket(responsePacket(advertiser.Service))[0].Metadata["ok"] != "changed" {
		t.Fatal("advertiser metadata was not mutable by owner")
	}
	for _, malformed := range [][]byte{nil, {0, 1, 2}, make([]byte, 12)} {
		if got := servicesFromPacket(malformed); got != nil {
			t.Fatalf("malformed packet produced services: %#v", got)
		}
	}
}
