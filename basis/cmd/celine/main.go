package main

import (
	"log"
	"net/http"
	"os"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/YumikoKawaii/celine/basis/gen/celine/v1/celinev1connect"
	"github.com/YumikoKawaii/celine/basis/internal/agent"
	"github.com/YumikoKawaii/celine/basis/internal/llm"
	"github.com/YumikoKawaii/celine/basis/internal/rpc"
)

func main() {
	addr := os.Getenv("CELINE_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is required (set it in env/.env — never commit it)")
	}
	brain := llm.New(apiKey, os.Getenv("CELINE_MODEL"))
	celine := agent.New(brain, agent.SystemPrompt())

	svc := rpc.NewCelineService(celine)
	path, handler := celinev1connect.NewCelineServiceHandler(svc)

	mux := http.NewServeMux()
	mux.Handle(path, handler)

	srv := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(devCORS(mux), &http2.Server{}),
	}

	log.Printf("celine backend listening on %s (CelineService at %s)", addr, path)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

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
