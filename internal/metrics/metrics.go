package metrics

import (
	"fmt"
	"io"
	"sync/atomic"
)

type Metrics struct { registrations atomic.Uint64; resolutions atomic.Uint64; jobsEnqueued atomic.Uint64; jobsCompleted atomic.Uint64; jobsFailed atomic.Uint64 }
func (m *Metrics) IncRegistrations(){m.registrations.Add(1)}
func (m *Metrics) IncResolutions(){m.resolutions.Add(1)}
func (m *Metrics) IncJobsEnqueued(){m.jobsEnqueued.Add(1)}
func (m *Metrics) IncJobsCompleted(){m.jobsCompleted.Add(1)}
func (m *Metrics) IncJobsFailed(){m.jobsFailed.Add(1)}
func (m *Metrics) WritePrometheus(w io.Writer) error {
	values:=[]struct{name,help string; value uint64}{{"atlasmesh_service_registrations_total","Total service registrations.",m.registrations.Load()},{"atlasmesh_service_resolutions_total","Total successful service resolutions.",m.resolutions.Load()},{"atlasmesh_jobs_enqueued_total","Total jobs accepted by the queue.",m.jobsEnqueued.Load()},{"atlasmesh_jobs_completed_total","Total jobs completed successfully.",m.jobsCompleted.Load()},{"atlasmesh_jobs_failed_total","Total job failure reports.",m.jobsFailed.Load()}}
	for _, metric:=range values { if _,err:=fmt.Fprintf(w,"# HELP %s %s\n# TYPE %s counter\n%s %d\n",metric.name,metric.help,metric.name,metric.name,metric.value);err!=nil{return err} }; return nil
}
