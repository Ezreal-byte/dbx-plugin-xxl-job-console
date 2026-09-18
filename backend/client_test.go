package main

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestMissingCredentialsFailBeforeNetwork(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
	}))
	defer srv.Close()
	s, err := newSession(connection{ID: "a", Config: config{BaseURL: srv.URL + "/custom/"}}, runtimeEndpoint{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	if err := s.probe(); err == nil || calls != 0 {
		t.Fatalf("missing credentials sent request: %v %d", err, calls)
	}
}

func transportProbe(s *session) error {
	_, e := s.request("/jobinfo/pageList", url.Values{"start": {"0"}, "length": {"1"}, "jobGroup": {"-1"}, "triggerStatus": {"-1"}}, false)
	return e
}

func TestHostPortBuildsServiceURLAndProbes(t *testing.T) {
	calls := 0
	authority := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/xxl-job-admin/jobinfo/pageList" || r.Host != authority {
			t.Errorf("unexpected request %s %s", r.Host, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credentials sent")
		}
		fmt.Fprint(w, `{"recordsTotal":0,"recordsFiltered":0,"data":[]}`)
	}))
	defer srv.Close()
	authority = strings.TrimPrefix(srv.URL, "http://")
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	for _, layers := range [][]transportLayer{nil, {{Type: "proxy"}}} {
		s, err := newSession(connection{Host: "127.0.0.1", Port: port, TransportLayers: layers}, runtimeEndpoint{Host: "127.0.0.1", Port: port})
		if err != nil {
			t.Fatal(err)
		}
		if s.base != srv.URL+"/xxl-job-admin" {
			t.Errorf("unexpected service URL %s", s.base)
		}
		if err := transportProbe(s); err != nil {
			t.Fatal(err)
		}
		s.close()
	}
	if calls != 2 {
		t.Fatalf("expected two readonly probes, got %d", calls)
	}
}

func TestHostPortOverridesLegacyAuthorityButPreservesSchemeAndPath(t *testing.T) {
	s, err := newSession(connection{Host: "new.internal", Port: 9443, Config: config{BaseURL: "https://old.internal:8443/custom/"}}, runtimeEndpoint{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	if s.base != "https://new.internal:9443/custom" {
		t.Fatalf("unexpected migrated address %s", s.base)
	}
}

func TestHostPortIPv6AndInvalidInputs(t *testing.T) {
	s, err := newSession(connection{Host: "::1", Port: 31957}, runtimeEndpoint{})
	if err != nil {
		t.Fatal(err)
	}
	if s.base != "http://[::1]:31957/xxl-job-admin" {
		t.Fatal(s.base)
	}
	s.close()
	for _, c := range []connection{{Host: "host", Port: 0}, {Host: "host", Port: 65536}, {Host: "host", Port: -1}, {Port: 31957}, {Host: "http://host", Port: 80}, {Host: "host:80", Port: 80}, {Host: "user@host", Port: 80}, {Host: "host/path", Port: 80}, {Host: "host?x", Port: 80}, {Host: "host#x", Port: 80}, {Host: "host%2fpath", Port: 80}} {
		if s, err := newSession(c, runtimeEndpoint{}); err == nil {
			s.close()
			t.Fatalf("unsafe host/port accepted: %+v", c)
		}
	}
}

func TestRejectUnsafeAddresses(t *testing.T) {
	for _, address := range []string{"", "file:///tmp/test", "http://u:p@localhost", "http://localhost?token=x", "http://localhost/#x", "http://localhost/a/../b", "http://localhost/%2e%2e/b", "http://localhost:0"} {
		t.Run(address, func(t *testing.T) {
			if s, e := newSession(connection{Config: config{BaseURL: address}}, runtimeEndpoint{}); e == nil {
				s.close()
				t.Fatal("unsafe address accepted")
			}
		})
	}
}

func TestRejectAuthenticationAndMalformedResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"redirect", "", 302}, {"unauthorized", "", 401}, {"forbidden", "", 403},
		{"loginhtml", `<html><form action="login"></form></html>`, 200},
		{"business", `{"code":500,"msg":"超出任务上限"}`, 200},
		{"malformed", `{"recordsTotal":1,"data":[]}`, 200},
		{"negativecount", `{"recordsTotal":-1,"recordsFiltered":-1,"data":[]}`, 200},
		{"huge", strings.Repeat("x", 4*1024*1024+1), 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "/toLogin")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			s, e := newSession(connection{Config: config{BaseURL: srv.URL}}, runtimeEndpoint{})
			if e != nil {
				t.Fatal(e)
			}
			defer s.close()
			if e = transportProbe(s); e == nil {
				t.Fatal("invalid service accepted")
			}
		})
	}
}

func TestWriteResultUnknownIsNotRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, `{"broken":true}`) }))
	defer srv.Close()
	s, e := newSession(connection{Config: config{BaseURL: srv.URL}}, runtimeEndpoint{})
	if e != nil {
		t.Fatal(e)
	}
	defer s.close()
	_, e = s.request("/jobinfo/trigger", url.Values{"id": {"7"}, "executorParam": {""}}, true)
	if e == nil || !strings.Contains(e.Error(), "结果未知") || calls != 1 {
		t.Fatalf("write retry or error missing: %d %v", calls, e)
	}
}

func TestTransportLayersDoNotSilentlyBypassHost(t *testing.T) {
	if s, e := newSession(connection{Config: config{BaseURL: "http://localhost"}, TransportLayers: []transportLayer{{Type: "ssh"}}}, runtimeEndpoint{}); e == nil {
		s.close()
		t.Fatal("transport silently bypassed")
	}
}

func TestHostTunnelPreservesServiceAuthorityAndPath(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(fmt.Sprint(secure), func(t *testing.T) {
			host, port, scheme := "service.invalid", 31957, "http"
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Host != net.JoinHostPort(host, fmt.Sprint(port)) || r.URL.Path != "/xxl-job-admin/jobinfo/pageList" {
					t.Errorf("service authority/path changed: %s %s", r.Host, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("service authentication added")
				}
				if secure && r.TLS.ServerName != host {
					t.Errorf("TLS SNI changed: %s", r.TLS.ServerName)
				}
				fmt.Fprint(w, `{"recordsTotal":0,"recordsFiltered":0,"data":[]}`)
			}))
			if secure {
				srv.StartTLS()
				host, port, scheme = srv.Certificate().DNSNames[0], 443, "https"
			} else {
				srv.Start()
			}
			defer srv.Close()
			u, _ := url.Parse(srv.URL)
			tunnelPort := srv.Listener.Addr().(*net.TCPAddr).Port
			var c connection
			data, _ := json.Marshal(map[string]any{"host": host, "port": port, "external_config": config{BaseURL: scheme + "://" + net.JoinHostPort(host, fmt.Sprint(port)) + "/xxl-job-admin/"}, "transport_layers": []any{map[string]any{"type": "proxy", "enabled": true, "profile_id": "shared"}}})
			if e := json.Unmarshal(data, &c); e != nil {
				t.Fatal(e)
			}
			s, e := newSession(c, runtimeEndpoint{Host: u.Hostname(), Port: tunnelPort})
			if e != nil {
				t.Fatal(e)
			}
			defer s.close()
			if secure {
				if s.client.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify {
					t.Fatal("TLS certificate verification disabled")
				}
				if e := transportProbe(s); e == nil {
					t.Fatal("untrusted tunnel certificate accepted")
				}
				roots := x509.NewCertPool()
				roots.AddCert(srv.Certificate())
				s.client.Transport.(*http.Transport).TLSClientConfig.RootCAs = roots
			}
			if e := transportProbe(s); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestTunnelFailureNeverFallsBackToService(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"recordsTotal":0,"recordsFiltered":0,"data":[]}`)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	servicePort := srv.Listener.Addr().(*net.TCPAddr).Port
	closed, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	tunnelPort := closed.Addr().(*net.TCPAddr).Port
	closed.Close()
	s, e := newSession(connection{Host: u.Hostname(), Port: servicePort, Config: config{BaseURL: srv.URL}, TransportLayers: []transportLayer{{Type: "proxy"}}}, runtimeEndpoint{Host: "127.0.0.1", Port: tunnelPort})
	if e != nil {
		t.Fatal(e)
	}
	defer s.close()
	if e := transportProbe(s); e == nil || calls != 0 {
		t.Fatalf("tunnel failure bypassed host: %v, direct calls=%d", e, calls)
	}
}

func TestDisabledLayersUseAddressDirectly(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"recordsTotal":0,"recordsFiltered":0,"data":[]}`)
	}))
	defer srv.Close()
	disabled := false
	s, e := newSession(connection{Config: config{BaseURL: srv.URL}, TransportLayers: []transportLayer{{Type: "proxy", Enabled: &disabled}}}, runtimeEndpoint{Host: "unreachable.invalid", Port: 1})
	if e != nil {
		t.Fatal(e)
	}
	defer s.close()
	if e := transportProbe(s); e != nil || calls != 1 {
		t.Fatalf("disabled layer must not route traffic: %v, calls=%d", e, calls)
	}
}

func TestTunnelRequiresMatchingTargetAndValidRuntime(t *testing.T) {
	for _, mutate := range []func(map[string]any, *runtimeEndpoint){
		func(c map[string]any, r *runtimeEndpoint) { c["host"] = "" },
		func(c map[string]any, r *runtimeEndpoint) { c["host"] = "http://wrong.invalid" },
		func(c map[string]any, r *runtimeEndpoint) { c["port"] = 0 },
		func(c map[string]any, r *runtimeEndpoint) { r.Host = "" },
		func(c map[string]any, r *runtimeEndpoint) { r.Port = 0 },
		func(c map[string]any, r *runtimeEndpoint) { r.Port = 65536 },
		func(c map[string]any, r *runtimeEndpoint) { r.Host = "http://localhost" },
	} {
		data := map[string]any{"host": "service.invalid", "port": 80, "external_config": config{BaseURL: "http://service.invalid/xxl-job-admin"}, "transport_layers": []any{map[string]any{"type": "proxy"}}}
		r := runtimeEndpoint{Host: "127.0.0.1", Port: 32123}
		mutate(data, &r)
		encoded, _ := json.Marshal(data)
		var copy connection
		if e := json.Unmarshal(encoded, &copy); e != nil {
			t.Fatal(e)
		}
		if s, e := newSession(copy, r); e == nil {
			s.close()
			t.Fatalf("invalid tunnel accepted: %+v %+v", copy, r)
		}
	}
}

func TestWriteHTMLOrRedirectDoesNotClaimNoMutation(t *testing.T) {
	for _, status := range []int{200, 302} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Location", "/login")
				w.WriteHeader(status)
				fmt.Fprint(w, `<html>Login</html>`)
			}))
			defer srv.Close()
			s, e := newSession(connection{Config: config{BaseURL: srv.URL}}, runtimeEndpoint{})
			if e != nil {
				t.Fatal(e)
			}
			defer s.close()
			_, e = s.request("/jobinfo/trigger", url.Values{"id": {"7"}}, true)
			if e == nil || !strings.Contains(e.Error(), "结果未知") || calls != 1 {
				t.Fatalf("ambiguous write must not retry or claim failure: %d %v", calls, e)
			}
		})
	}
}
