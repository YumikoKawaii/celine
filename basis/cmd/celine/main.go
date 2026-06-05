package main

import (
	"log"
	"net/http"
	"os"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/YumikoKawaii/celine/basis/gen/celine/v1/celinev1connect"
	"github.com/YumikoKawaii/celine/basis/internal/rpc"
)

func main() {
	addr := os.Getenv("CELINE_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	svc := rpc.NewCelineService()
	path, handler := celinev1connect.NewCelineServiceHandler(svc)

	mux := http.NewServeMux()
	mux.Handle(path, handler)

	srv := &http.Server{
		Addr: addr,
		// h2c lets gRPC clients speak HTTP/2 cleartext; the browser uses the
		// Connect protocol over HTTP/1.1 on the same handler. devCORS opens it
		// up for the Vite dev server during step 1.
		Handler: h2c.NewHandler(devCORS(mux), &http2.Server{}),
	}

	log.Printf("celine backend listening on %s (CelineService at %s)", addr, path)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// devCORS is a permissive CORS shim for local development so the Vite dev
// server (a different origin) can call the Connect endpoint. Tighten or drop
// this once the app is served from a single origin.
func devCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Connect-Protocol-Version, Connect-Timeout-Ms, X-User-Agent, X-Grpc-Web, Authorization")
		w.Header().Set("Access-Control-Expose-Headers",
			"Grpc-Status, Grpc-Message, Grpc-Status-Details-Bin")
		w.Header().Set("Access-Control-Max-Age", "7200")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}
