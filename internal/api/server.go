package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mkarson1997/karzoun-atlasmesh/internal/cluster"
	"github.com/mkarson1997/karzoun-atlasmesh/internal/jobs"
	"github.com/mkarson1997/karzoun-atlasmesh/internal/metrics"
	"github.com/mkarson1997/karzoun-atlasmesh/internal/registry"
)

const maxBodyBytes = 1 << 20

type Server struct {
	Registry *registry.Registry
	Jobs     *jobs.Queue
	Cluster  *cluster.Membership
	Metrics  *metrics.Metrics
	Logger   *slog.Logger
}

func NewServer(r *registry.Registry, q *jobs.Queue, c *cluster.Membership, m *metrics.Metrics, logger *slog.Logger) *Server {
	if r == nil { r = registry.New() }
	if q == nil { q = jobs.New() }
	if c == nil { c = cluster.New() }
	if m == nil { m = &metrics.Metrics{} }
	if logger == nil { logger = slog.Default() }
	return &Server{Registry:r, Jobs:q, Cluster:c, Metrics:m, Logger:logger}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	mux.HandleFunc("GET /metrics", s.prometheus)
	mux.HandleFunc("POST /v1/services/register", s.registerService)
	mux.HandleFunc("POST /v1/services/{service}/{id}/heartbeat", s.heartbeatService)
	mux.HandleFunc("POST /v1/services/{service}/{id}/health", s.setServiceHealth)
	mux.HandleFunc("GET /v1/services", s.listServices)
	mux.HandleFunc("GET /v1/services/{service}/resolve", s.resolveService)
	mux.HandleFunc("POST /v1/jobs", s.enqueueJob)
	mux.HandleFunc("POST /v1/jobs/claim", s.claimJob)
	mux.HandleFunc("GET /v1/jobs/{id}", s.getJob)
	mux.HandleFunc("POST /v1/jobs/{id}/heartbeat", s.heartbeatJob)
	mux.HandleFunc("POST /v1/jobs/{id}/complete", s.completeJob)
	mux.HandleFunc("POST /v1/jobs/{id}/fail", s.failJob)
	mux.HandleFunc("POST /v1/nodes/register", s.registerNode)
	mux.HandleFunc("POST /v1/nodes/{id}/heartbeat", s.heartbeatNode)
	mux.HandleFunc("GET /v1/cluster/status", s.clusterStatus)
	return s.logging(mux)
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.Logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) { writeJSON(w,http.StatusOK,map[string]string{"status":"ok"}) }
func (s *Server) readyz(w http.ResponseWriter, _ *http.Request) { writeJSON(w,http.StatusOK,map[string]string{"status":"ready"}) }
func (s *Server) prometheus(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Content-Type","text/plain; version=0.0.4; charset=utf-8"); if err:=s.Metrics.WritePrometheus(w);err!=nil{s.Logger.Error("write metrics","error",err)} }

type registerServiceRequest struct { Service string `json:"service"`; ID string `json:"id"`; Address string `json:"address"`; Weight int `json:"weight"`; TTLSeconds int `json:"ttl_seconds"`; Metadata map[string]string `json:"metadata,omitempty"` }

func (s *Server) registerService(w http.ResponseWriter,r *http.Request){var req registerServiceRequest;if !decodeJSON(w,r,&req){return};instance,err:=s.Registry.Register(registry.RegisterInput{Service:req.Service,ID:req.ID,Address:req.Address,Weight:req.Weight,TTL:time.Duration(req.TTLSeconds)*time.Second,Metadata:req.Metadata});if err!=nil{writeError(w,http.StatusBadRequest,err);return};s.Metrics.IncRegistrations();writeJSON(w,http.StatusCreated,instance)}
func (s *Server) heartbeatService(w http.ResponseWriter,r *http.Request){ttl,ok:=ttlFromBody(w,r);if !ok{return};instance,err:=s.Registry.Heartbeat(r.PathValue("service"),r.PathValue("id"),ttl);if errors.Is(err,registry.ErrNotFound){writeError(w,http.StatusNotFound,err);return};if err!=nil{writeError(w,http.StatusBadRequest,err);return};writeJSON(w,http.StatusOK,instance)}
func (s *Server) setServiceHealth(w http.ResponseWriter,r *http.Request){var req struct{Healthy bool `json:"healthy"`};if !decodeJSON(w,r,&req){return};err:=s.Registry.SetHealth(r.PathValue("service"),r.PathValue("id"),req.Healthy);if errors.Is(err,registry.ErrNotFound){writeError(w,http.StatusNotFound,err);return};if err!=nil{writeError(w,http.StatusBadRequest,err);return};writeJSON(w,http.StatusOK,map[string]any{"healthy":req.Healthy})}
func (s *Server) listServices(w http.ResponseWriter,r *http.Request){writeJSON(w,http.StatusOK,map[string]any{"services":s.Registry.List(r.URL.Query().Get("service"))})}
func (s *Server) resolveService(w http.ResponseWriter,r *http.Request){instance,err:=s.Registry.Resolve(r.PathValue("service"));if errors.Is(err,registry.ErrNoHealthyNode){writeError(w,http.StatusServiceUnavailable,err);return};if err!=nil{writeError(w,http.StatusInternalServerError,err);return};s.Metrics.IncResolutions();writeJSON(w,http.StatusOK,instance)}

func (s *Server) enqueueJob(w http.ResponseWriter,r *http.Request){var req struct{ID string `json:"id"`;Type string `json:"type"`;Payload json.RawMessage `json:"payload,omitempty"`;MaxAttempts int `json:"max_attempts"`};if !decodeJSON(w,r,&req){return};job,err:=s.Jobs.Enqueue(jobs.Job{ID:req.ID,Type:req.Type,Payload:req.Payload,MaxAttempts:req.MaxAttempts});if errors.Is(err,jobs.ErrDuplicateJob){writeError(w,http.StatusConflict,err);return};if err!=nil{writeError(w,http.StatusBadRequest,err);return};s.Metrics.IncJobsEnqueued();writeJSON(w,http.StatusCreated,job)}
func (s *Server) claimJob(w http.ResponseWriter,r *http.Request){var req struct{Worker string `json:"worker"`;LeaseSeconds int `json:"lease_seconds"`};if !decodeJSON(w,r,&req){return};job,err:=s.Jobs.Claim(req.Worker,time.Duration(req.LeaseSeconds)*time.Second);if errors.Is(err,jobs.ErrNoWork){w.WriteHeader(http.StatusNoContent);return};if err!=nil{writeError(w,http.StatusBadRequest,err);return};writeJSON(w,http.StatusOK,job)}
func (s *Server) getJob(w http.ResponseWriter,r *http.Request){job,err:=s.Jobs.Get(r.PathValue("id"));if errors.Is(err,jobs.ErrNotFound){writeError(w,http.StatusNotFound,err);return};if err!=nil{writeError(w,http.StatusInternalServerError,err);return};writeJSON(w,http.StatusOK,job)}
func (s *Server) heartbeatJob(w http.ResponseWriter,r *http.Request){var req struct{Worker string `json:"worker"`;LeaseSeconds int `json:"lease_seconds"`};if !decodeJSON(w,r,&req){return};job,err:=s.Jobs.Heartbeat(r.PathValue("id"),req.Worker,time.Duration(req.LeaseSeconds)*time.Second);if err!=nil{writeJobMutationError(w,err);return};writeJSON(w,http.StatusOK,job)}
func (s *Server) completeJob(w http.ResponseWriter,r *http.Request){var req struct{Worker string `json:"worker"`;Result json.RawMessage `json:"result,omitempty"`};if !decodeJSON(w,r,&req){return};job,err:=s.Jobs.Complete(r.PathValue("id"),req.Worker,req.Result);if err!=nil{writeJobMutationError(w,err);return};s.Metrics.IncJobsCompleted();writeJSON(w,http.StatusOK,job)}
func (s *Server) failJob(w http.ResponseWriter,r *http.Request){var req struct{Worker string `json:"worker"`;Error string `json:"error"`;RetryAfterSeconds int `json:"retry_after_seconds"`};if !decodeJSON(w,r,&req){return};job,err:=s.Jobs.Fail(r.PathValue("id"),req.Worker,req.Error,time.Duration(req.RetryAfterSeconds)*time.Second);if err!=nil{writeJobMutationError(w,err);return};s.Metrics.IncJobsFailed();writeJSON(w,http.StatusOK,job)}
func writeJobMutationError(w http.ResponseWriter,err error){switch{case errors.Is(err,jobs.ErrNotFound):writeError(w,http.StatusNotFound,err);case errors.Is(err,jobs.ErrLeaseLost):writeError(w,http.StatusConflict,err);default:writeError(w,http.StatusBadRequest,err)}}

func (s *Server) registerNode(w http.ResponseWriter,r *http.Request){var req struct{ID string `json:"id"`;Address string `json:"address"`;Zone string `json:"zone"`;TTLSeconds int `json:"ttl_seconds"`};if !decodeJSON(w,r,&req){return};node,err:=s.Cluster.Register(req.ID,req.Address,req.Zone,time.Duration(req.TTLSeconds)*time.Second);if err!=nil{writeError(w,http.StatusBadRequest,err);return};writeJSON(w,http.StatusCreated,node)}
func (s *Server) heartbeatNode(w http.ResponseWriter,r *http.Request){ttl,ok:=ttlFromBody(w,r);if !ok{return};node,err:=s.Cluster.Heartbeat(r.PathValue("id"),ttl);if errors.Is(err,cluster.ErrNodeNotFound){writeError(w,http.StatusNotFound,err);return};if err!=nil{writeError(w,http.StatusBadRequest,err);return};writeJSON(w,http.StatusOK,node)}
func (s *Server) clusterStatus(w http.ResponseWriter,_ *http.Request){services:=s.Registry.List("");nodes:=s.Cluster.List();queued:=len(s.Jobs.List(jobs.Queued));leased:=len(s.Jobs.List(jobs.Leased));dead:=len(s.Jobs.List(jobs.DeadLetter));writeJSON(w,http.StatusOK,map[string]any{"nodes":nodes,"node_count":len(nodes),"service_instance_count":len(services),"jobs":map[string]int{"queued":queued,"leased":leased,"dead_letter":dead}})}

func ttlFromBody(w http.ResponseWriter,r *http.Request)(time.Duration,bool){var req struct{TTLSeconds int `json:"ttl_seconds"`};if !decodeJSON(w,r,&req){return 0,false};return time.Duration(req.TTLSeconds)*time.Second,true}
func decodeJSON(w http.ResponseWriter,r *http.Request,dst any)bool{r.Body=http.MaxBytesReader(w,r.Body,maxBodyBytes);decoder:=json.NewDecoder(r.Body);decoder.DisallowUnknownFields();if err:=decoder.Decode(dst);err!=nil{writeError(w,http.StatusBadRequest,fmt.Errorf("invalid json: %w",err));return false};var extra any;if err:=decoder.Decode(&extra);err!=io.EOF{writeError(w,http.StatusBadRequest,errors.New("request must contain one JSON value"));return false};return true}
func writeJSON(w http.ResponseWriter,status int,value any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.WriteHeader(status);_=json.NewEncoder(w).Encode(value)}
func writeError(w http.ResponseWriter,status int,err error){writeJSON(w,status,map[string]string{"error":sanitizeError(err)})}
func sanitizeError(err error)string{if err==nil{return "unknown error"};msg:=strings.TrimSpace(err.Error());if len(msg)>512{msg=msg[:512]};return msg}
