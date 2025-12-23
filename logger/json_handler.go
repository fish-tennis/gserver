package logger

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
)

type JsonHandlerWithStdOutput struct {
	*slog.JSONHandler
	// 标准输出,用于调试
	stdLogger *log.Logger
	stdPrefix string
}

func NewJsonHandlerWithStdOutput(w io.Writer, opts *slog.HandlerOptions, useStdOutput bool) *JsonHandlerWithStdOutput {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	h := &JsonHandlerWithStdOutput{
		JSONHandler: slog.NewJSONHandler(w, opts),
	}
	if useStdOutput {
		h.stdLogger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
	}
	return h
}

func (h *JsonHandlerWithStdOutput) WithAttrs(attrs []slog.Attr) slog.Handler {
	handler := &JsonHandlerWithStdOutput{
		JSONHandler: h.JSONHandler.WithAttrs(attrs).(*slog.JSONHandler),
		stdLogger:   h.stdLogger,
	}
	if h.stdLogger != nil {
		for _, attr := range attrs {
			handler.stdPrefix += " " + attr.String()
		}
	}
	return handler
}

func (h *JsonHandlerWithStdOutput) WithGroup(name string) slog.Handler {
	return &JsonHandlerWithStdOutput{
		JSONHandler: h.JSONHandler.WithGroup(name).(*slog.JSONHandler),
		stdLogger:   h.stdLogger,
	}
}

func (h *JsonHandlerWithStdOutput) Handle(ctx context.Context, r slog.Record) error {
	// 用于调试时输出
	if h.stdLogger != nil {
		builder := strings.Builder{}
		builder.WriteString("[")
		builder.WriteString(string(r.Level.String()[0]))
		builder.WriteString("] ")
		builder.WriteString(r.Message)
		if h.stdPrefix != "" {
			builder.WriteString(h.stdPrefix)
		}
		r.Attrs(func(attr slog.Attr) bool {
			builder.WriteString(" ")
			builder.WriteString(attr.String())
			return true
		})
		h.stdLogger.Output(4, builder.String())
	}
	return h.JSONHandler.Handle(ctx, r)
}

func GetShortFileName(file string) string {
	idx := strings.LastIndexByte(file, '/')
	if idx >= 0 {
		idx = strings.LastIndexByte(file[:idx], '/')
		if idx >= 0 {
			return file[idx+1:] // 让source简短些
		}
	}
	return file
}
