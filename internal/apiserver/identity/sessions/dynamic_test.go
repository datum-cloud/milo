package sessions

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	identityv1alpha1 "go.miloapis.com/milo/pkg/apis/identity/v1alpha1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	authuser "k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
)

// --- helpers ---

func writePEM(dir, name string, b []byte) (string, error) {
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return "", err
	}
	return p, nil
}

// makeSelfSignedServerAndCA spins up a TLS server with a self-signed cert,
// returns the server and a CA PEM that trusts it. Also captures the SNI the
// client sent in the handshake via a channel.
func makeSelfSignedServerAndCA(t *testing.T, handler http.Handler) (*httptest.Server, []byte, <-chan string) {
	t.Helper()

	// Generate a CA
	caKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "test-ca",
			Organization: []string{"test"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	// Server cert (issued by CA)
	srvKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	host := "127.0.0.1"
	ip := net.ParseIP(host)

	srvTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "test-server",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		IPAddresses: []net.IP{ip},
		DNSNames:    []string{"localhost", "test.internal"},
	}
	caCert, _ := x509.ParseCertificate(caDER)
	srvDER, _ := x509.CreateCertificate(rand.Reader, srvTmpl, caCert, &srvKey.PublicKey, caKey)

	srvCert := tls.Certificate{
		Certificate: [][]byte{srvDER, caDER}, // include chain
		PrivateKey:  srvKey,
	}

	// Capture SNI
	sniCh := make(chan string, 1)

	ts := httptest.NewUnstartedServer(handler)
	ts.TLS = &tls.Config{
		Certificates: []tls.Certificate{srvCert},
		// Observe SNI
		GetConfigForClient: func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
			select {
			case sniCh <- chi.ServerName:
			default:
			}
			// return nil to use the default config above
			return nil, nil
		},
	}
	ts.StartTLS()

	return ts, caPEM, sniCh
}

// minimalUnstructuredList encodes an empty list for a resource.
func minimalUnstructuredList() *unstructured.UnstructuredList {
	ul := &unstructured.UnstructuredList{}
	ul.SetAPIVersion("example.com/v1")
	ul.SetKind("WidgetList")
	return ul
}

// --- tests ---

func TestNewDynamicProvider_TLSConfig(t *testing.T) {
	tmp := t.TempDir()

	// dummy client cert/key (not required by the server in this test)
	ckey, _ := rsa.GenerateKey(rand.Reader, 2048)
	ckeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(ckey)})
	certTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(100),
		Subject:      pkix.Name{CommonName: "client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, certTmpl, certTmpl, &ckey.PublicKey, ckey)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	certFile, _ := writePEM(tmp, "client.crt", certPEM)
	keyFile, _ := writePEM(tmp, "client.key", ckeyPEM)

	// fake CA path (we won’t actually use it against a real server here)
	caFile, _ := writePEM(tmp, "ca.crt", []byte("-----BEGIN CERTIFICATE-----\nMIIB...dummy\n-----END CERTIFICATE-----\n"))

	providerURL := "https://api.staging.env.datum.net:30443"

	dp, err := NewDynamicProvider(Config{
		ProviderURL:    providerURL,
		ProviderGVR:    schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"},
		CAFile:         caFile,
		ClientCertFile: certFile,
		ClientKeyFile:  keyFile,
		Timeout:        15 * time.Second,
		ExtrasAllow:    map[string]struct{}{"scopes": {}},
	})
	if err != nil {
		t.Fatalf("NewDynamicProvider: %v", err)
	}

	if dp.base.Host != providerURL {
		t.Fatalf("Host = %q, want %q", dp.base.Host, providerURL)
	}
	if got, want := dp.base.TLSClientConfig.ServerName, "api.staging.env.datum.net"; got != want {
		t.Fatalf("ServerName = %q, want %q", got, want)
	}
	if dp.base.TLSClientConfig.Insecure {
		t.Fatal("Insecure should be false")
	}
	if dp.base.TLSClientConfig.CAFile != caFile {
		t.Fatalf("CAFile = %q, want %q", dp.base.TLSClientConfig.CAFile, caFile)
	}
	if dp.base.TLSClientConfig.CertFile != certFile {
		t.Fatalf("CertFile = %q, want %q", dp.base.TLSClientConfig.CertFile, certFile)
	}
	if dp.base.TLSClientConfig.KeyFile != keyFile {
		t.Fatalf("KeyFile = %q, want %q", dp.base.TLSClientConfig.KeyFile, keyFile)
	}
}

func TestDynForUser_SendsAuthProxyHeadersAndTLS(t *testing.T) {
	// Choose a GVR we’ll serve
	gvr := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"}

	// Capture request headers
	var gotUser string
	var gotGroups []string
	var gotExtra map[string][]string

	// Minimal fake API implementing:
	//  GET /apis/example.com/v1/widgets
	mux := http.NewServeMux()
	mux.HandleFunc("/apis/"+gvr.Group+"/"+gvr.Version+"/"+gvr.Resource, func(w http.ResponseWriter, r *http.Request) {
		gotUser = r.Header.Get("X-Remote-User")
		gotGroups = r.Header.Values("X-Remote-Group")

		gotExtra = map[string][]string{}
		for k, vals := range r.Header {
			if strings.HasPrefix(http.CanonicalHeaderKey(k), "X-Remote-Extra-") {
				// Preserve original case-insensitivity by normalizing key
				gotExtra[k] = vals
			}
		}

		ul := minimalUnstructuredList()
		w.Header().Set("Content-Type", "application/json")
		_ = unstructured.UnstructuredJSONScheme.Encode(ul, w)
	})

	// Stand up TLS server w/ custom CA and capture SNI
	ts, caPEM, sniCh := makeSelfSignedServerAndCA(t, mux)
	defer ts.Close()

	url := ts.URL
	wantSNI := "test.internal"

	tmp := t.TempDir()
	caFile, _ := writePEM(tmp, "ca.crt", caPEM)

	// Create provider that talks to our TLS server
	dp, err := NewDynamicProvider(Config{
		ProviderURL: url,
		ProviderGVR: gvr,
		CAFile:      caFile,
		// client cert optional for this test; server doesn't require it
		Timeout: 5 * time.Second,
		// only allow the "scopes" extra to flow through
		ExtrasAllow: map[string]struct{}{"scopes": {}},
	})
	if err != nil {
		t.Fatalf("NewDynamicProvider: %v", err)
	}
	dp.base.TLSClientConfig.ServerName = "test.internal"

	// Build a user in context
	u := &authuser.DefaultInfo{
		Name:   "jane@example.com",
		UID:    "abcd-1234",
		Groups: []string{"dev", "admins"},
		Extra: map[string][]string{
			"scopes": {"read:widgets", "list:widgets"},
			"other":  {"SHOULD_NOT_PASS"},
		},
	}
	ctx := apirequest.WithUser(context.Background(), u)

	// Call a real method that triggers an HTTP GET to our server
	out, err := dp.ListSessions(ctx, u, nil)
	if err != nil {
		t.Fatalf("ListSessions error: %v", err)
	}
	if out == nil {
		t.Fatalf("expected non-nil SessionList")
	}

	// Assert headers
	if gotUser != "jane@example.com" {
		t.Fatalf("X-Remote-User = %q, want %q", gotUser, "jane@example.com")
	}
	// Order isn’t guaranteed; check membership
	wantGroups := map[string]bool{"dev": true, "admins": true}
	if len(gotGroups) != 2 || !wantGroups[gotGroups[0]] && !wantGroups[gotGroups[1]] {
		t.Fatalf("X-Remote-Group = %v, want both dev and admins", gotGroups)
	}
	// Extras: only allowed key should be present (case-insensitive header map)
	var foundScopes bool
	for k, vals := range gotExtra {
		if strings.EqualFold(k, "X-Remote-Extra-Scopes") {
			foundScopes = true
			if len(vals) != 2 {
				t.Fatalf("X-Remote-Extra-Scopes values = %v, want 2", vals)
			}
		}
		// ensure "other" didn't slip through
		if strings.EqualFold(k, "X-Remote-Extra-Other") {
			t.Fatalf("unexpected X-Remote-Extra-Other present")
		}
	}
	if !foundScopes {
		t.Fatalf("X-Remote-Extra-Scopes header not found")
	}

	// Assert SNI was set to the URL hostname
	select {
	case gotSNI := <-sniCh:
		if gotSNI != wantSNI {
			t.Fatalf("SNI(ServerName) = %q, want %q", gotSNI, wantSNI)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("did not observe ClientHello/SNI")
	}
}

// Sanity check that retry loop doesn’t blow up: server errors once, then succeeds.
func TestListSessions_Retries(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"}

	var hits int
	mux := http.NewServeMux()
	mux.HandleFunc("/apis/"+gvr.Group+"/"+gvr.Version+"/"+gvr.Resource, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			http.Error(w, "try again", http.StatusInternalServerError)
			return
		}
		ul := minimalUnstructuredList()
		w.Header().Set("Content-Type", "application/json")
		_ = unstructured.UnstructuredJSONScheme.Encode(ul, w)
	})

	ts := httptest.NewTLSServer(mux)
	defer ts.Close()

	// trust the httptest server cert
	caPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ts.Certificate().Raw, // <- parsed x509 cert, use Raw
	})
	tmp := t.TempDir()
	caFile, _ := writePEM(tmp, "ca.crt", caPEM)

	dp, err := NewDynamicProvider(Config{
		ProviderURL: ts.URL,
		ProviderGVR: gvr,
		CAFile:      caFile,
		Retries:     1,
	})
	if err != nil {
		t.Fatalf("NewDynamicProvider: %v", err)
	}

	u := &authuser.DefaultInfo{Name: "x"}
	ctx := apirequest.WithUser(context.Background(), u)

	got, err := dp.ListSessions(ctx, u, nil)
	if err != nil {
		t.Fatalf("ListSessions error after retry: %v", err)
	}
	if got == nil {
		t.Fatalf("expected non-nil SessionList")
	}
	if hits != 2 {
		t.Fatalf("expected 2 hits (1 error + 1 success), got %d", hits)
	}
}

// compile-time “uses” to avoid import pruning during refactors
var _ = identityv1alpha1.SessionList{}
