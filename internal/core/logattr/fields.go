package logattr

import (
	"log/slog"
	"time"
)

const (
	keyUserID         = "user_id"
	keyAttemptedID    = "attempted_id"
	keySubscriptionID = "subscription_id"
	keyTaskID         = "task_id"
	keyTaskType       = "task_type"
	keyMethod         = "method"
	keyPath           = "path"
	keyHTTPStatus     = "http_status"
	keyStatus         = "status"
	keyErrorCode      = "error_code"
	keyTraceID        = "trace_id"
	keySpanID         = "span_id"
	keyError          = "error"
	keyEnv            = "env"
	keyPort           = "port"
	keyInterval       = "interval"
	keyConcurrency    = "concurrency"
	keyStartupTime    = "startup_time"
	keySubscription   = "subscription"
	keyTemplate       = "template"
	keyKey            = "key"
	keyRemaining      = "remaining"
	keyValidTill      = "valid_till"
	keyProcessAt      = "process_at"
	keyService        = "service"
	keyJaeger         = "jaeger"
	keyIP             = "ip"
	keyMessage        = "message"
	keyDaysBefore     = "days_before"
	keyTotal          = "total"
	keySuccess        = "success"
	keyFailed         = "failed"
	keyHost           = "host"
	keyDatabase       = "database"
	keyRedisDB        = "redis_db"
	keyQueue          = "queue"
	keyRenewalDate    = "renewal_date"
	keyConfigFile     = "config_file"
	keyOtelEnabled    = "otel_enabled"

	// Rate Limiter
	keyRate   = "rate"
	keyBurst  = "burst"
	keyPeriod = "period"
	keyPrefix = "prefix"

	// JWT
	keyIssuer             = "issuer"
	keyAccessExpiryHours  = "access_expiry_hours"
	keyRefreshExpiryHours = "refresh_expiry_hours"

	// Scheduler
	keySchedulerName = "scheduler_name"
	keyReminderDays  = "reminder_days"
	keyStartupDelay  = "startup_delay"
	keyEnabledForEnv = "enabled_for_env"

	// Queue Worker
	keyWorkerName = "worker_name"

	// HTTP
	keyTimeout    = "request_timeout"
	keyTLSEnabled = "tls_enabled"

	// Miscellaneous
	keyPodName = "pod_name"
)

// UserID returns an slog.Attr for the user ID.
func UserID(id string) slog.Attr {
	return slog.String(keyUserID, id)
}

// AttemptedID returns an slog.Attr for the attempted user ID.
func AttemptedID(id string) slog.Attr {
	return slog.String(keyAttemptedID, id)
}

// SubscriptionID returns an slog.Attr for the subscription ID.
func SubscriptionID(id string) slog.Attr {
	return slog.String(keySubscriptionID, id)
}

// TaskID returns an slog.Attr for the task ID.
func TaskID(id string) slog.Attr {
	return slog.String(keyTaskID, id)
}

// TaskType returns an slog.Attr for the task type.
func TaskType(t string) slog.Attr {
	return slog.String(keyTaskType, t)
}

// Method returns an slog.Attr for the HTTP method.
func Method(m string) slog.Attr {
	return slog.String(keyMethod, m)
}

// Path returns an slog.Attr for the HTTP path.
func Path(p string) slog.Attr {
	return slog.String(keyPath, p)
}

// HTTPStatus returns an slog.Attr for the HTTP status code.
func HTTPStatus(s int) slog.Attr {
	return slog.Int(keyHTTPStatus, s)
}

// Status returns an slog.Attr for the generic status (e.g., subscription status).
func Status(s string) slog.Attr {
	return slog.String(keyStatus, s)
}

// ErrorCode returns an slog.Attr for the application error code.
func ErrorCode(c string) slog.Attr {
	return slog.String(keyErrorCode, c)
}

// TraceID returns an slog.Attr for the OpenTelemetry trace ID.
func TraceID(tid string) slog.Attr {
	return slog.String(keyTraceID, tid)
}

// SpanID returns an slog.Attr for the OpenTelemetry span ID.
func SpanID(sid string) slog.Attr {
	return slog.String(keySpanID, sid)
}

// Error returns an slog.Attr for the error.
func Error(err error) slog.Attr {
	return slog.Any(keyError, err)
}

// Env returns an slog.Attr for the environment.
func Env(e string) slog.Attr {
	return slog.String(keyEnv, e)
}

// Port returns an slog.Attr for the port.
func Port(p int) slog.Attr {
	return slog.Int(keyPort, p)
}

// Interval returns an slog.Attr for the duration interval.
func Interval(i time.Duration) slog.Attr {
	return slog.Duration(keyInterval, i)
}

// Concurrency returns an slog.Attr for the concurrency level.
func Concurrency(c int) slog.Attr {
	return slog.Int(keyConcurrency, c)
}

// StartupTime returns an slog.Attr for the service startup time.
func StartupTime(d time.Duration) slog.Attr {
	return slog.Duration(keyStartupTime, d)
}

// SubscriptionName returns an slog.Attr for the subscription name.
func SubscriptionName(n string) slog.Attr {
	return slog.String(keySubscription, n)
}

// Template returns an slog.Attr for the email template name.
func Template(t string) slog.Attr {
	return slog.String(keyTemplate, t)
}

// Key returns an slog.Attr for the generic key.
func Key(k string) slog.Attr {
	return slog.String(keyKey, k)
}

// Remaining returns an slog.Attr for the remaining count.
func Remaining(r int) slog.Attr {
	return slog.Int(keyRemaining, r)
}

// ValidTill returns an slog.Attr for the validity date.
func ValidTill(t time.Time) slog.Attr {
	return slog.Time(keyValidTill, t)
}

// ProcessAt returns an slog.Attr for the scheduled processing time.
func ProcessAt(t time.Time) slog.Attr {
	return slog.Time(keyProcessAt, t)
}

// Service returns an slog.Attr for the service name.
func Service(s string) slog.Attr {
	return slog.String(keyService, s)
}

// Jaeger returns an slog.Attr for the Jaeger endpoint.
func Jaeger(j string) slog.Attr {
	return slog.String(keyJaeger, j)
}

// IP returns an slog.Attr for the IP address.
func IP(ip string) slog.Attr {
	return slog.String(keyIP, ip)
}

// Message returns an slog.Attr for the message text.
func Message(m string) slog.Attr {
	return slog.String(keyMessage, m)
}

// DaysBefore returns an slog.Attr for the number of days before an event.
func DaysBefore(d int) slog.Attr {
	return slog.Int(keyDaysBefore, d)
}

// Total returns an slog.Attr for the total count of items.
func Total(c int) slog.Attr {
	return slog.Int(keyTotal, c)
}

// Success returns an slog.Attr for the count of items.
func Success(c int) slog.Attr {
	return slog.Int(keySuccess, c)
}

// Failed returns an slog.Attr for the count of items.
func Failed(c int) slog.Attr {
	return slog.Int(keyFailed, c)
}

// Host returns an slog.Attr for the host name.
func Host(h string) slog.Attr {
	return slog.String(keyHost, h)
}

// Database returns an slog.Attr for the database name.
func Database(d string) slog.Attr {
	return slog.String(keyDatabase, d)
}

// RedisDB returns an slog.Attr for the Redis database number.
func RedisDB(d int) slog.Attr {
	return slog.Int(keyRedisDB, d)
}

// Rate returns an slog.Attr for the rate value.
func Rate(r int) slog.Attr {
	return slog.Int(keyRate, r)
}

// Burst returns an slog.Attr for the burst value.
func Burst(b int) slog.Attr {
	return slog.Int(keyBurst, b)
}

// Period returns an slog.Attr for the duration period.
func Period(p time.Duration) slog.Attr {
	return slog.Duration(keyPeriod, p)
}

// Queue returns an slog.Attr for the queue name.
func Queue(q string) slog.Attr {
	return slog.String(keyQueue, q)
}

// RenewalDate returns an slog.Attr for the renewal date.
func RenewalDate(t time.Time) slog.Attr {
	return slog.Time(keyRenewalDate, t)
}

// ConfigFile returns an slog.Attr for the config file path.
func ConfigFile(f string) slog.Attr {
	return slog.String(keyConfigFile, f)
}

// OtelEnabled returns an slog.Attr for the OpenTelemetry enabled status.
func OtelEnabled(b bool) slog.Attr {
	return slog.Bool(keyOtelEnabled, b)
}

// Prefix returns an slog.Attr for the prefix.
func Prefix(p string) slog.Attr {
	return slog.String(keyPrefix, p)
}

// Issuer returns an slog.Attr for the issuer.
func Issuer(i string) slog.Attr {
	return slog.String(keyIssuer, i)
}

// AccessExpiryHours returns an slog.Attr for the access expiry hours.
func AccessExpiryHours(h int) slog.Attr {
	return slog.Int(keyAccessExpiryHours, h)
}

// RefreshExpiryHours returns an slog.Attr for the refresh expiry hours.
func RefreshExpiryHours(h int) slog.Attr {
	return slog.Int(keyRefreshExpiryHours, h)
}

// ReminderDays returns an slog.Attr for the reminder days.
func ReminderDays(d []int) slog.Attr {
	return slog.Any(keyReminderDays, d)
}

// StartupDelay returns an slog.Attr for the startup delay.
func StartupDelay(d time.Duration) slog.Attr {
	return slog.Duration(keyStartupDelay, d)
}

// EnabledForEnv returns an slog.Attr for the enabled environments.
func EnabledForEnv(e []string) slog.Attr {
	return slog.Any(keyEnabledForEnv, e)
}

// SchedulerName returns an slog.Attr for the scheduler name.
func SchedulerName(n string) slog.Attr {
	return slog.String(keySchedulerName, n)
}

// WorkerName returns an slog.Attr for the worker name.
func WorkerName(n string) slog.Attr {
	return slog.String(keyWorkerName, n)
}

// PodName returns an slog.Attr for the pod name.
func PodName(n string) slog.Attr {
	return slog.String(keyPodName, n)
}

// Timeout returns an slog.Attr for the timeout duration.
func Timeout(d time.Duration) slog.Attr {
	return slog.Duration(keyTimeout, d)
}

// TLSEnabled returns an slog.Attr for the TLS enabled status.
func TLSEnabled(b bool) slog.Attr {
	return slog.Bool(keyTLSEnabled, b)
}
