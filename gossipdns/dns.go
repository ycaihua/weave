package gossipdns

import (
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/miekg/dns"

	"github.com/weaveworks/weave/common"
)

type DNSServer struct {
	ns      *Nameserver
	domain  string
	servers []*dns.Server
}

func NewDNSServer(ns *Nameserver, domain string, port int) (*DNSServer, error) {
	s := &DNSServer{
		ns:     ns,
		domain: domain,
	}
	err := s.listen(port)
	return s, err
}

func (d *DNSServer) listen(port int) error {
	udpListener, err := net.ListenPacket("udp", ":"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	udpServer := &dns.Server{PacketConn: udpListener, Handler: d.createMux()}

	tcpListener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		udpServer.Shutdown()
		return err
	}
	tcpServer := &dns.Server{Listener: tcpListener, Handler: d.createMux()}

	d.servers = []*dns.Server{udpServer, tcpServer}
	return nil
}

func (d *DNSServer) ActivateAndServe() {
	for _, server := range d.servers {
		go func(server *dns.Server) {
			server.ActivateAndServe()
		}(server)
	}
}

func (d *DNSServer) Stop() error {
	for _, server := range d.servers {
		if err := server.Shutdown(); err != nil {
			return err
		}
	}
	return nil
}

func (d *DNSServer) createMux() *dns.ServeMux {
	m := dns.NewServeMux()
	m.HandleFunc(d.domain, func(w dns.ResponseWriter, req *dns.Msg) {
		common.Info.Printf("dns request: %+v", *req)
		if len(req.Question) != 1 || req.Question[0].Qtype != dns.TypeA {
			return
		}

		hostname := req.Question[0].Name
		addrs := d.ns.Lookup(hostname)

		response := dns.Msg{}
		response.RecursionAvailable = true
		response.Authoritative = true
		response.SetReply(req)
		response.Answer = make([]dns.RR, len(addrs))

		header := &dns.RR_Header{
			Name:   hostname,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    uint32(60),
		}

		for i, addr := range addrs {
			ip := addr.IP4()
			response.Answer[i] = &dns.A{Hdr: *header, A: ip}
		}
		shuffleAnswers(&response.Answer)

		common.Info.Printf("dns response: %+v", response)
		err := w.WriteMsg(&response)
		if err != nil {
			common.Info.Printf("err: %v", err)
		}
	})
	return m
}

func shuffleAnswers(answers *[]dns.RR) {
	if len(*answers) <= 1 {
		return
	}

	for i := range *answers {
		j := rand.Intn(i + 1)
		(*answers)[i], (*answers)[j] = (*answers)[j], (*answers)[i]
	}
}

func (d *DNSServer) HandleHTTP(router *mux.Router) {
	router.Methods("GET").Path("/domain").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, d.domain)
	})
}
