package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vestigiumincaligne/polka/internal/converter"
	"github.com/vestigiumincaligne/polka/internal/store"
)

const (
	formatOriginal    = "original"
	formatEPUB3       = "epub3"
	conversionTimeout = 180 * time.Second
)

var (
	errAttachmentSize  = errors.New("empty or oversized attachment")
	errDeliveryLibrary = errors.New("library is not configured")
)

type deliveryAttachment struct {
	title, name string
	data        []byte
}

func isFB2(extension string) bool {
	return strings.EqualFold(strings.TrimPrefix(extension, "."), "fb2")
}

// readDeliveryFile applies the limit to actual bytes, not just catalog metadata.
func readDeliveryFile(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || int64(len(data)) > limit {
		return nil, errAttachmentSize
	}
	return data, nil
}

// deliveryTitle uses the snapshot of the file being sent, even if an import
// replaces the catalog while conversion is running.
func deliveryTitle(file *store.BookFile) string {
	for _, value := range []string{file.Title, file.File} {
		if title := strings.Join(strings.Fields(value), " "); title != "" {
			return title
		}
	}
	return "book-" + strconv.FormatInt(file.ID, 10)
}

func (s *Server) prepareDelivery(ctx context.Context, file *store.BookFile, format string) (deliveryAttachment, error) {
	var attachment deliveryAttachment
	limit := int64(maxSendBytes)
	if format == formatEPUB3 {
		limit = maxSendSourceBytes
	}
	data, err := s.deliverySource(ctx, file, limit)
	if err != nil {
		return attachment, err
	}
	extension := strings.TrimPrefix(file.Ext, ".")
	if format == formatEPUB3 {
		data, err = convertDelivery(ctx, data)
		if err != nil {
			return attachment, err
		}
		extension = "epub"
	}
	title := deliveryTitle(file)
	if authors := strings.Join(strings.Fields(file.AuthorNames), " "); authors != "" {
		title = authors + " - " + title
	}
	return deliveryAttachment{title: title, name: sanitizeFileName(title) + "." + extension, data: data}, nil
}

// Finish reading and close the library/archive before running the converter.
func (s *Server) deliverySource(ctx context.Context, file *store.BookFile, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.lib == nil {
		return nil, errDeliveryLibrary
	}
	reader, size, err := s.lib.Open(file.Folder, file.File, file.Ext)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	if size > limit {
		return nil, errAttachmentSize
	}
	return readDeliveryFile(reader, limit)
}

func convertDelivery(ctx context.Context, sourceData []byte) ([]byte, error) {
	dir, err := os.MkdirTemp("", "polka-convert-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	source := filepath.Join(dir, "source.fb2")
	if err := os.WriteFile(source, sourceData, 0600); err != nil {
		return nil, err
	}
	binary := os.Getenv("POLKA_FBC_BINARY")
	if binary == "" {
		binary = "fbc"
	}
	output, err := converter.EPUB3(ctx, binary, source, conversionTimeout)
	if err != nil {
		return nil, err
	}
	reader, err := os.Open(output)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return readDeliveryFile(reader, maxSendBytes)
}

func (s *Server) deliveryError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errDeliveryLibrary):
		http.Error(w, tr(reqLang(r), "send.noLibrary"), http.StatusConflict)
	case errors.Is(err, errAttachmentSize):
		http.Error(w, tr(reqLang(r), "send.large"), http.StatusRequestEntityTooLarge)
	case errors.Is(err, context.Canceled):
		return // The client has gone away; temporary files have already been removed.
	case errors.Is(err, context.DeadlineExceeded):
		http.Error(w, tr(reqLang(r), "send.convertFail"), http.StatusGatewayTimeout)
	case errors.Is(err, converter.ErrFailed):
		http.Error(w, tr(reqLang(r), "send.convertFail"), http.StatusUnprocessableEntity)
	default:
		s.apiError(w, err)
	}
}
