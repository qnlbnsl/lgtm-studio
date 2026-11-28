package link

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/sysinfo"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

type reportProbes struct {
	now       func() time.Time
	readFacts func(clientVersion string) (sysinfo.Facts, error)
	readLoad  func() (sysinfo.Load, error)
	scan      func(context.Context, harness.ScanOptions) []harness.Found
}

type reportKind uint8

const (
	factsReport reportKind = iota
	harnessReport
	reportKindCount
)

type snapshot struct {
	order uint64
	event *linkv1.Event
}

type reportQueue struct {
	committed uint64
	pending   snapshot
}

type reporter struct {
	settings config.Settings
	version  string
	options  harness.ScanOptions
	append   func(context.Context, *linkv1.Event) (uint64, error)
	logger   *slog.Logger
	probes   reportProbes
	order    atomic.Uint64

	// The probes run independently. The publisher serializes appends and logs.
	mu     sync.Mutex
	queues [reportKindCount]reportQueue
	fault  error
}

var errReadFacts = errors.New("link: could not read machine facts")

func newReporter(settings config.Settings, version string, options harness.ScanOptions, appendReport func([]byte) (uint64, error), logger *slog.Logger, probes reportProbes) *reporter {
	return newReporterWithContextAppend(settings, version, options, func(_ context.Context, payload []byte) (uint64, error) {
		return appendReport(payload)
	}, logger, probes)
}

func newReporterWithContextAppend(settings config.Settings, version string, options harness.ScanOptions, appendReport func(context.Context, []byte) (uint64, error), logger *slog.Logger, probes reportProbes) *reporter {
	return newReporterWithEventAppend(settings, version, options, func(ctx context.Context, event *linkv1.Event) (uint64, error) {
		payload, err := (*OutgoingFilter)(nil).encodeFilteredReport(event)
		if err != nil {
			return 0, err
		}
		return appendReport(ctx, payload)
	}, logger, probes)
}

func newReporterWithEventAppend(settings config.Settings, version string, options harness.ScanOptions, appendReport func(context.Context, *linkv1.Event) (uint64, error), logger *slog.Logger, probes reportProbes) *reporter {
	if probes.now == nil {
		probes.now = time.Now
	}
	if probes.readFacts == nil {
		probes.readFacts = sysinfo.ReadFacts
	}
	if probes.readLoad == nil {
		probes.readLoad = sysinfo.ReadLoad
	}
	if probes.scan == nil {
		probes.scan = harness.Scan
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &reporter{
		settings: settings, version: version, options: options,
		append: appendReport, logger: logger, probes: probes,
	}
}

func (r *reporter) captureFacts() error {
	order := r.order.Add(1)
	captured := timestamppb.New(r.probes.now())
	facts, err := r.probes.readFacts(r.version)
	if err != nil {
		return errReadFacts
	}
	event := &linkv1.Event{
		HappenedAt: captured,
		Report: &linkv1.Event_Facts{Facts: &linkv1.Facts{
			Os:            facts.OS,
			Cores:         max(facts.Cores, 1),
			MemoryBytes:   facts.MemoryBytes,
			ClientVersion: facts.ClientVersion,
			Workspace:     reportWorkspace(r.settings.RepositoriesFolder, r.options.Home),
			AgentLimit:    r.settings.AgentLimit,
		}},
	}
	_, err = r.publish(context.Background(), factsReport, order, event, false)
	return err
}

func (r *reporter) heartbeat(approved bool) *linkv1.Heartbeat {
	beat := new(linkv1.Heartbeat)
	if !approved {
		return beat
	}
	load, err := r.probes.readLoad()
	if err != nil {
		return beat
	}
	if load.CPUPercent <= 100 {
		beat.CpuPercent = proto.Uint32(load.CPUPercent)
	}
	if load.MemoryPercent <= 100 {
		beat.MemoryPercent = proto.Uint32(load.MemoryPercent)
	}
	return beat
}

func (r *reporter) scanHarnesses(ctx context.Context, requested bool) (bool, error) {
	if ctx.Err() != nil {
		return false, nil
	}
	order := r.order.Add(1)
	captured := timestamppb.New(r.probes.now())
	found := r.probes.scan(ctx, r.options)
	versions := make(map[string]string, len(found))
	for _, row := range found {
		if row.Installed {
			versions[string(row.Harness)] = row.Version
		}
	}
	defs := harness.Definitions()
	rows := make([]*linkv1.Harness, 0, len(defs))
	for _, def := range defs {
		row := &linkv1.Harness{Name: string(def.Name)}
		if version := versions[row.Name]; version != "" {
			row.Version = proto.String(version)
		}
		rows = append(rows, row)
	}
	event := &linkv1.Event{
		HappenedAt: captured,
		Report: &linkv1.Event_HarnessesFound{HarnessesFound: &linkv1.HarnessesFound{
			Harnesses: rows,
		}},
	}
	return r.publish(ctx, harnessReport, order, event, requested)
}

func (r *reporter) publish(ctx context.Context, kind reportKind, order uint64, event *linkv1.Event, requested bool) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fault != nil {
		return false, r.fault
	}
	queue := &r.queues[kind]
	if ctx.Err() != nil || order <= queue.committed {
		return false, nil
	}
	candidate := snapshot{order: order, event: event}
	committed, err := r.appendSnapshot(ctx, queue, candidate)
	if err == context.Canceled || err == context.DeadlineExceeded || errors.Is(err, errScreeningUnavailable) {
		return false, nil
	}
	if !committed && err == nil && !requested && order > queue.pending.order {
		queue.pending = candidate
	}
	return committed, err
}

func (r *reporter) flush() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fault != nil {
		return r.fault
	}
	first, second := &r.queues[factsReport], &r.queues[harnessReport]
	if first.pending.order > second.pending.order {
		first, second = second, first
	}
	for _, queue := range []*reportQueue{first, second} {
		if queue.pending.order != 0 {
			if _, err := r.appendSnapshot(context.Background(), queue, queue.pending); err != nil && !errors.Is(err, errScreeningUnavailable) {
				return err
			}
		}
	}
	return nil
}

func (r *reporter) appendSnapshot(ctx context.Context, queue *reportQueue, candidate snapshot) (bool, error) {
	sequence, err := r.append(ctx, candidate.event)
	if errors.Is(err, ErrOutboxFull) {
		return false, nil
	}
	if err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded || errors.Is(err, errScreeningUnavailable) {
			return false, err
		}
		if errors.Is(err, ErrStoreFailed) {
			err = ErrStoreFailed
		}
		// A failed transaction may have reached disk. Fence further appends.
		r.fault = err
		return false, err
	}
	queue.committed = candidate.order
	if queue.pending.order <= candidate.order {
		queue.pending = snapshot{}
	}
	r.logger.Info("link_event_buffered", "sequence", sequence)
	return true, nil
}

func reportString(value string, limit int) string {
	return truncateReportString(normalizeReportString(value), limit)
}

func normalizeReportString(value string) string {
	return strings.ReplaceAll(strings.ToValidUTF8(value, ""), "\x00", "")
}

func truncateReportString(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	for !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit]
}

func reportWorkspace(folder, home string) string {
	if filepath.IsAbs(home) && filepath.IsAbs(folder) {
		rel, err := filepath.Rel(home, folder)
		switch {
		case err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)):
		case rel == ".":
			folder = "~"
		default:
			folder = "~/" + filepath.ToSlash(rel)
		}
	}
	folder = reportString(folder, len(folder))
	if len(folder) > 256 {
		return ""
	}
	return folder
}
