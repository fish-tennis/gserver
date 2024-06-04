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
	slog.JSONHandler
	// 标准输出,用于调试
	stdLogger *log.Logger
}

func NewJsonHandlerWithStdOutput(w io.Writer, opts *slog.HandlerOptions, useStdOutput bool) *JsonHandlerWithStdOutput {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	h := &JsonHandlerWithStdOutput{
		JSONHandler: *slog.NewJSONHandler(w, opts),
	}
	if useStdOutput {
		h.stdLogger = log.New(os.Stdout, "", log.LstdFlags|log.Llongfile)
	}
	return h
}

func (h *JsonHandlerWithStdOutput) Handle(ctx context.Context, r slog.Record) error {
	// 用于调试时输出
	if h.stdLogger != nil {
		builder := strings.Builder{}
		builder.WriteString("[")
		builder.WriteString(string(r.Level.String()[0]))
		builder.WriteString("] ")
		builder.WriteString(r.Message)
		r.Attrs(func(attr slog.Attr) bool {
			builder.WriteString(" ")
			builder.WriteString(attr.String())
			return true
		})
		h.stdLogger.Output(4, builder.String())
	}
	return h.JSONHandler.Handle(ctx, r)
}
