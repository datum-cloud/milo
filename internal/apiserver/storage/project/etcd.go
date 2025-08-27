// etcd.go
package projectstorage

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	storagebackend "k8s.io/apiserver/pkg/storage/storagebackend"
)

func newEtcdClientFrom(cfg storagebackend.Config) (*clientv3.Client, error) {
	// cfg.ServerList is the list of etcd endpoints used by the store
	ends := cfg.Transport.ServerList
	if len(ends) == 0 {
		// optional: fall back to env for convenience
		if env := os.Getenv("ETCD_SERVERS"); env != "" {
			ends = splitCSV(env)
		}
	}

	tlsCfg, err := tlsFromFiles(cfg.Transport.TrustedCAFile, cfg.Transport.CertFile, cfg.Transport.KeyFile)
	if err != nil {
		return nil, err
	}
	return clientv3.New(clientv3.Config{
		Endpoints:   ends,
		TLS:         tlsCfg,
		DialTimeout: 10 * time.Second,
	})
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func tlsFromFiles(caFile, certFile, keyFile string) (*tls.Config, error) {
	var certs []tls.Certificate
	if certFile != "" && keyFile != "" {
		c, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}
	var roots *x509.CertPool
	if caFile != "" {
		b, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		roots = x509.NewCertPool()
		roots.AppendCertsFromPEM(b)
	}
	return &tls.Config{
		RootCAs:      roots,
		Certificates: certs,
		MinVersion:   tls.VersionTLS12,
	}, nil
}
