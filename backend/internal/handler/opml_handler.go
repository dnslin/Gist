package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"gist/backend/internal/service"
	"gist/backend/pkg/logger"
)

const maxOPMLSize = 5 << 20

type OPMLHandler struct {
	service        service.OPMLService
	taskManager    service.ImportTaskService
	writerLauncher service.WriterLauncher
}

func NewOPMLHandler(opmlService service.OPMLService, taskManager service.ImportTaskService, writerLauncher service.WriterLauncher) *OPMLHandler {
	return &OPMLHandler{service: opmlService, taskManager: taskManager, writerLauncher: writerLauncher}
}

func (h *OPMLHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/opml/import", h.Import)
	g.DELETE("/opml/import", h.CancelImport)
	g.GET("/opml/import/status", h.ImportStatus)
	g.GET("/opml/export", h.Export)
}

// Import imports subscriptions from an OPML file.
// @Summary Import OPML
// @Description Start importing feeds and folders from an OPML file
// @Tags opml
// @Accept multipart/form-data
// @Accept xml
// @Produce json
// @Param file formData file false "OPML file to import"
// @Success 200 {object} importStartedResponse
// @Failure 400 {object} errorResponse
// @Failure 413 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /opml/import [post]
func (h *OPMLHandler) Import(c echo.Context) error {
	req := c.Request()
	req.Body = http.MaxBytesReader(c.Response().Writer, req.Body, maxOPMLSize)

	var reader io.Reader
	if strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/") {
		file, err := c.FormFile("file")
		if err != nil {
			if err == http.ErrMissingFile {
				return c.JSON(http.StatusBadRequest, errorResponse{Error: "missing file"})
			}
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request"})
		}
		if file.Size > maxOPMLSize {
			return c.JSON(http.StatusRequestEntityTooLarge, errorResponse{Error: "file too large"})
		}
		src, err := file.Open()
		if err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request"})
		}
		defer src.Close()
		reader = io.LimitReader(src, maxOPMLSize)
	} else {
		reader = io.LimitReader(req.Body, maxOPMLSize)
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		logger.Warn("opml import read failed", "module", "handler", "action", "import", "resource", "opml", "result", "failed", "error", err)
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "read file failed"})
	}

	total := h.countFeedsInOPML(bytes.NewReader(content))
	reservation, err := h.writerLauncher.ReserveWriter(req.Context(), service.WriterRequestBound)
	if err != nil {
		logger.Warn("opml import admission rejected", "module", "handler", "action", "import", "resource", "opml", "result", "rejected", "error", err)
		return writeServiceError(c, err)
	}
	defer reservation.Release()

	_, taskCtx := h.taskManager.Start(total, reservation.Context())
	reservation.Publish()
	reservation.Launch(func(writerCtx context.Context) {
		h.runImport(writerCtx, taskCtx, content)
	})
	logger.Info("opml import started", "module", "handler", "action", "import", "resource", "opml", "result", "ok", "count", len(content))
	return c.JSON(http.StatusOK, importStartedResponse{Status: "started"})
}

func (h *OPMLHandler) runImport(writerCtx, taskCtx context.Context, content []byte) {
	onProgress := func(p service.ImportProgress) { h.taskManager.Update(p.Current, p.Feed) }
	result, followUp, err := h.service.Import(taskCtx, bytes.NewReader(content), onProgress)
	if err != nil {
		if taskCtx.Err() != nil {
			logger.Warn("opml import cancelled", "module", "handler", "action", "import", "resource", "opml", "result", "cancelled")
			return
		}
		logger.Error("opml import failed", "module", "handler", "action", "import", "resource", "opml", "result", "failed", "error", err)
		h.taskManager.Fail(err)
		return
	}
	if taskCtx.Err() != nil {
		logger.Warn("opml import cancelled", "module", "handler", "action", "import", "resource", "opml", "result", "cancelled")
		return
	}

	h.taskManager.Complete(result)
	logger.Info("opml import completed", "module", "handler", "action", "import", "resource", "opml", "result", "ok", "folders_created", result.FoldersCreated, "folders_skipped", result.FoldersSkipped, "feeds_created", result.FeedsCreated, "feeds_skipped", result.FeedsSkipped)
	h.service.RunImportFollowUp(writerCtx, followUp)
}

func (h *OPMLHandler) countFeedsInOPML(reader io.Reader) int {
	content, err := io.ReadAll(reader)
	if err != nil {
		return 0
	}
	return bytes.Count(bytes.ToLower(content), []byte("xmlurl"))
}

// CancelImport cancels the current import task.
// @Summary Cancel Import
// @Description Cancel the current import task
// @Tags opml
// @Produce json
// @Success 200 {object} importCancelledResponse
// @Router /opml/import [delete]
func (h *OPMLHandler) CancelImport(c echo.Context) error {
	cancelled := h.taskManager.Cancel()
	logger.Info("opml import cancel", "module", "handler", "action", "import", "resource", "opml", "result", "ok", "cancelled", cancelled)
	return c.JSON(http.StatusOK, importCancelledResponse{Cancelled: cancelled})
}

// ImportStatus returns the current import task status via SSE.
// @Summary Import Status
// @Description Get current import task status via SSE stream
// @Tags opml
// @Produce text/event-stream
// @Success 200 {object} service.ImportTask
// @Router /opml/import/status [get]
func (h *OPMLHandler) ImportStatus(c echo.Context) error {
	res := c.Response()
	res.Header().Set("Content-Type", "text/event-stream")
	res.Header().Set("Cache-Control", "no-cache")
	res.Header().Set("Connection", "keep-alive")
	res.WriteHeader(http.StatusOK)

	ctx := c.Request().Context()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	h.sendTaskStatus(res)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			task := h.taskManager.Get()
			h.sendTaskStatus(res)
			if task != nil && (task.Status == "done" || task.Status == "error" || task.Status == "cancelled") {
				return nil
			}
		}
	}
}

func (h *OPMLHandler) sendTaskStatus(res *echo.Response) {
	task := h.taskManager.Get()
	if task == nil {
		data, _ := json.Marshal(importIdleResponse{Status: "idle"})
		fmt.Fprintf(res, "data: %s\n\n", data)
	} else {
		data, _ := json.Marshal(task)
		fmt.Fprintf(res, "data: %s\n\n", data)
	}
	res.Flush()
}

// Export exports subscriptions to an OPML file.
// @Summary Export OPML
// @Description Export all feeds and folders to an OPML file
// @Tags opml
// @Produce xml
// @Success 200 {string} string "OPML file content"
// @Router /opml/export [get]
func (h *OPMLHandler) Export(c echo.Context) error {
	payload, err := h.service.Export(c.Request().Context())
	if err != nil {
		logger.Error("opml export failed", "module", "handler", "action", "export", "resource", "opml", "result", "failed", "error", err)
		return writeServiceError(c, err)
	}
	logger.Info("opml export", "module", "handler", "action", "export", "resource", "opml", "result", "ok")
	c.Response().Header().Set("Content-Disposition", `attachment; filename="gist.opml"`)
	return c.Blob(http.StatusOK, "application/xml", payload)
}
