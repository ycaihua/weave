package gossipdns

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/miekg/dns"

	"github.com/weaveworks/weave/common"
	"github.com/weaveworks/weave/ipam/address"
)

func badRequest(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadRequest)
	common.Warning.Println("[nameserver]:", err.Error())
}

func (n *Nameserver) HandleHTTP(router *mux.Router) {
	router.Methods("PUT").Path("/name/{hostname}/{ip}/{container}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			vars      = mux.Vars(r)
			hostname  = vars["hostname"]
			container = vars["container"]
			ipStr     = vars["ip"]
			ip, err   = address.ParseIP(ipStr)
		)
		if err != nil {
			badRequest(w, err)
			return
		}

		entry := Entry{
			Hostname:    dns.Fqdn(hostname),
			Addr:        ip,
			Origin:      n.ourName,
			ContainerID: container,
		}

		if err := n.AddEntry(entry); err != nil {
			badRequest(w, fmt.Errorf("Unable to add entry: %s", err))
			return
		}

		w.WriteHeader(204)
	})

	// Want to delete by hostname, ip, container id, or any combination.
	// Therefore allow users to specify * for dimensions they don't know.
	router.Methods("DELETE").Path("/name/{hostname}/{ip}/{container}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			vars      = mux.Vars(r)
			hostname  = vars["hostname"]
			container = vars["container"]
			ipStr     = vars["ip"]
			ip, err   = address.ParseIP(ipStr)
		)
		if err != nil && ipStr != "*" {
			badRequest(w, err)
			return
		}
		if hostname != "*" {
			hostname = dns.Fqdn(hostname)
		}

		n.Delete(hostname, container, ipStr, ip)
		w.WriteHeader(204)
	})

	router.Methods("GET").Path("/name/{hostname}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			vars     = mux.Vars(r)
			hostname = vars["hostname"]
			addrs    = n.Lookup(hostname)
			ips      = []string{}
		)

		if len(addrs) == 0 {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		for _, a := range addrs {
			ips = append(ips, a.String())
		}
		if err := json.NewEncoder(w).Encode(ips); err != nil {
			badRequest(w, fmt.Errorf("Unable to add entry: %s", err))
			return
		}
	})
}
