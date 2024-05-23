package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/taosdata/taoskeeper/db"
	"github.com/taosdata/taoskeeper/util"
)

func TestHttps(t *testing.T) {
	server := startProxy()
	defer server.Shutdown(context.Background())

	cfg := util.GetCfg()
	cfg.TDengine.Usessl = true
	cfg.TDengine.Port = 34443

	CreateDatabase(cfg.TDengine.Username, cfg.TDengine.Password, cfg.TDengine.Host, cfg.TDengine.Port, cfg.TDengine.Usessl, cfg.Metrics.Database, cfg.Metrics.DatabaseOptions)

	gm := NewGeneralMetric(cfg)
	err := gm.Init(router)
	assert.NoError(t, err)

	testcfg := struct {
		name   string
		ts     int64
		tbname string
		data   string
		expect string
	}{
		name:   "1",
		tbname: "taosd_cluster_basic",
		ts:     1705655770381,
		data:   `{"ts":"1705655770381","cluster_id":"7648966395564416484","protocol":2,"first_ep":"ssfood06:6130","first_ep_dnode_id":1,"cluster_version":"3.2.1.0.alp"}`,
		expect: "7648966395564416484",
	}

	conn, err := db.NewConnectorWithDb(gm.username, gm.password, gm.host, gm.port, gm.database, gm.usessl)
	assert.NoError(t, err)
	defer func() {
		_, _ = conn.Query(context.Background(), fmt.Sprintf("drop database if exists %s", gm.database))
	}()

	t.Run(testcfg.name, func(t *testing.T) {
		w := httptest.NewRecorder()
		body := strings.NewReader(testcfg.data)
		req, _ := http.NewRequest(http.MethodPost, "/taosd-cluster-basic", body)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)

		data, err := conn.Query(context.Background(), fmt.Sprintf("select ts, cluster_id from %s.%s where ts=%d", gm.database, testcfg.tbname, testcfg.ts))
		assert.NoError(t, err)
		assert.Equal(t, 1, len(data.Data))
		assert.Equal(t, testcfg.expect, data.Data[0][1])
	})

}

func generateSelfSignedCert() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P384(), crand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumber, err := crand.Int(crand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Your Company"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(crand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	keyPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyPEM})

	return tls.X509KeyPair(certPEM, keyPEMBlock)
}

func startProxy() *http.Server {
	// Generate self-signed certificate
	cert, err := generateSelfSignedCert()
	if err != nil {
		log.Fatalf("Failed to generate self-signed certificate: %v", err)
	}

	target := "http://127.0.0.1:6041"
	proxyURL, err := url.Parse(target)
	if err != nil {
		log.Fatalf("Failed to parse target URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(proxyURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		http.Error(w, "Proxy error", http.StatusBadGateway)
	}
	mux := http.NewServeMux()
	mux.Handle("/", proxy)

	server := &http.Server{
		Addr:      ":34443",
		Handler:   mux,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		// Setup server timeouts for better handling of idle connections and slowloris attacks
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Println("Starting server on :34443")
	go func() {
		err = server.ListenAndServeTLS("", "")
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTPS server: %v", err)
		}
	}()
	return server
}
