package singtun

import (
	"net"
	"net/netip"

	"github.com/miekg/dns"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

// answerNCSIProbe replies to a Windows connectivity lookup with a fixed public
// address.
//
// Windows NCSI treats a successful resolution of dns.msftncsi.com as proof of
// connectivity and only then sends the HTTP probe, which the proxy answers
// locally. A fake-ip cannot be used here because 198.18.0.0/15 is reserved for
// benchmarking rather than public space, and Windows rejects it outright.
//
// Handing back a fixed public address keeps the adapter status correct without
// depending on any upstream resolver being reachable.
func (h *Handler) answerNCSIProbe(msg *dns.Msg, domain string, qtype uint16, destination M.Socksaddr, writer N.PacketWriter) {
	addr := netip.MustParseAddr(ncsiStaticIP)

	resp := new(dns.Msg)
	resp.SetReply(msg)
	resp.RecursionAvailable = true

	header := func(rrtype uint16) dns.RR_Header {
		return dns.RR_Header{
			Name:   domain,
			Rrtype: rrtype,
			Class:  dns.ClassINET,
			Ttl:    300,
		}
	}

	// The probe is IPv4 only; answer NOERROR with no records for AAAA so the
	// resolver does not treat the empty section as a failure.
	switch qtype {
	case dns.TypeA:
		resp.Answer = append(resp.Answer, &dns.A{
			Hdr: header(dns.TypeA),
			A:   net.IP(addr.AsSlice()).To4(),
		})
	case dns.TypeAAAA:
		resp.Answer = append(resp.Answer, &dns.AAAA{
			Hdr:  header(dns.TypeAAAA),
			AAAA: net.IP(addr.AsSlice()).To16(),
		})
	default:
		return
	}

	raw, err := resp.Pack()
	if err != nil {
		h.logf("[sing-tun] failed to pack NCSI answer: " + err.Error())
		return
	}

	packet := buf.NewPacket()
	packet.Write(raw)
	if err := writer.WritePacket(packet, destination); err != nil {
		packet.Release()
		h.logf("[sing-tun] failed to write NCSI answer: " + err.Error())
	}
}
