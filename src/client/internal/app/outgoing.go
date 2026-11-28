package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/link"
)

const maxClaudeSignInBytes = 64 << 10

func claudeSignInReader(getenv func(string) string) func() []string {
	dir := getenv("CLAUDE_CONFIG_DIR")
	if dir == "" {
		home := getenv("HOME")
		if home == "" {
			return nil
		}
		dir = filepath.Join(home, ".claude")
	}
	path := filepath.Join(dir, ".credentials.json")
	return func() []string {
		file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxClaudeSignInBytes {
			return nil
		}
		data, err := io.ReadAll(io.LimitReader(file, maxClaudeSignInBytes+1))
		if err != nil || len(data) > maxClaudeSignInBytes {
			return nil
		}
		var credentials struct {
			OAuth struct {
				AccessToken  string `json:"accessToken"`
				RefreshToken string `json:"refreshToken"`
			} `json:"claudeAiOauth"`
		}
		if json.Unmarshal(data, &credentials) != nil {
			return nil
		}
		return secretForms(credentials.OAuth.AccessToken, credentials.OAuth.RefreshToken)
	}
}

func secretForms(values ...string) []string {
	var forms []string
	for _, value := range values {
		if value == "" {
			continue
		}
		raw := []byte(value)
		forms = append(forms, value, hex.EncodeToString(raw),
			base64.StdEncoding.EncodeToString(raw), base64.RawStdEncoding.EncodeToString(raw),
			base64.URLEncoding.EncodeToString(raw), base64.RawURLEncoding.EncodeToString(raw))
	}
	return forms
}

type outgoingHandler struct {
	handler slog.Handler
	filter  *link.OutgoingFilter
	scopes  []logScope
}

type logScope struct {
	group string
	attrs []slog.Attr
}

func newOutgoingLogger(output io.Writer, filter *link.OutgoingFilter) *slog.Logger {
	return slog.New(&outgoingHandler{handler: slog.NewJSONHandler(output, nil), filter: filter})
}

func (h *outgoingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *outgoingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.withScope(logScope{attrs: slices.Clone(attrs)})
}

func (h *outgoingHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return h.withScope(logScope{group: name})
}

func (h *outgoingHandler) withScope(scope logScope) slog.Handler {
	next := *h
	next.scopes = append(slices.Clone(h.scopes), scope)
	return &next
}

func (h *outgoingHandler) Handle(ctx context.Context, record slog.Record) error {
	// Resolve user-provided values before locking the filter.
	attrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		attr.Value = resolveLogValue(attr.Value)
		attrs = append(attrs, attr)
		return true
	})
	scopes := slices.Clone(h.scopes)
	for i := range scopes {
		scopes[i].attrs = resolveLogAttrs(scopes[i].attrs)
	}

	var err error
	h.filter.WithTextMask(func(mask func(string) string) {
		handler := h.handler
		screened := slog.NewRecord(record.Time, record.Level, mask(record.Message), record.PC)
		for _, scope := range scopes {
			if scope.group != "" {
				handler = handler.WithGroup(mask(scope.group))
			} else {
				handler = handler.WithAttrs(maskLogAttrs(scope.attrs, mask))
			}
		}
		screened.AddAttrs(maskLogAttrs(attrs, mask)...)
		err = handler.Handle(ctx, screened)
	})
	return err
}

func resolveLogAttrs(attrs []slog.Attr) []slog.Attr {
	resolved := slices.Clone(attrs)
	for i := range resolved {
		resolved[i].Value = resolveLogValue(resolved[i].Value)
	}
	return resolved
}

func resolveLogValue(value slog.Value) slog.Value {
	value = value.Resolve()
	switch value.Kind() {
	case slog.KindGroup:
		return slog.GroupValue(resolveLogAttrs(value.Group())...)
	case slog.KindTime:
		return slog.StringValue(value.Time().Format(time.RFC3339Nano))
	case slog.KindAny:
		return resolveJSONLogValue(value.Any())
	default:
		return value
	}
}

func resolveJSONLogValue(data any) (value slog.Value) {
	unavailable := slog.StringValue("[unavailable]")
	defer func() {
		if recover() != nil {
			value = unavailable
		}
	}()
	if err, ok := data.(error); ok {
		if _, marshals := data.(json.Marshaler); !marshals {
			return slog.StringValue(err.Error())
		}
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return unavailable
	}
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if decoder.Decode(&decoded) != nil {
		return unavailable
	}
	return slog.AnyValue(decoded)
}

func maskLogAttrs(attrs []slog.Attr, mask func(string) string) []slog.Attr {
	for i := range attrs {
		attr := &attrs[i]
		attr.Key = mask(attr.Key)
		switch value := attr.Value; value.Kind() {
		case slog.KindString:
			attr.Value = slog.StringValue(mask(value.String()))
		case slog.KindGroup:
			attr.Value = slog.GroupValue(maskLogAttrs(value.Group(), mask)...)
		case slog.KindAny:
			attr.Value = slog.AnyValue(maskJSONValue(value.Any(), mask))
		}
	}
	return attrs
}

func maskJSONValue(value any, mask func(string) string) any {
	switch value := value.(type) {
	case string:
		return mask(value)
	case map[string]any:
		masked := make(map[string]any, len(value))
		for key, item := range value {
			masked[mask(key)] = maskJSONValue(item, mask)
		}
		return masked
	case []any:
		for i := range value {
			value[i] = maskJSONValue(value[i], mask)
		}
	}
	return value
}
