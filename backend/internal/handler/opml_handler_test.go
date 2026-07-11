package handler_test

import (
	"context"
	"errors"
	"gist/backend/internal/handler"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gist/backend/internal/service"
	"gist/backend/internal/service/mock"
)

func TestOPMLHandler_ImportPublishesTaskBeforeHTTP200(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockOPML := mock.NewMockOPMLService(ctrl)
	mockTask := mock.NewMockImportTaskService(ctrl)
	started := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan struct{})

	mockTask.EXPECT().Start(1, gomock.Any()).DoAndReturn(func(int, context.Context) (string, context.Context) {
		close(started)
		return "task-id", context.Background()
	})
	mockOPML.EXPECT().Import(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, io.Reader, func(service.ImportProgress)) (service.ImportResult, service.ImportFollowUp, error) {
		<-release
		return service.ImportResult{}, service.ImportFollowUp{}, nil
	})
	mockTask.EXPECT().Complete(service.ImportResult{})
	mockOPML.EXPECT().RunImportFollowUp(gomock.Any(), service.ImportFollowUp{}).Do(func(context.Context, service.ImportFollowUp) { close(completed) })

	h := handler.NewOPMLHandlerHelper(mockOPML, mockTask, writerLauncherFunc(reserveTestWriter))
	e := newTestEcho()
	req := newJSONRequest(http.MethodPost, "/opml/import", nil)
	req.Body = &ioReaderCloser{s: `<?xml version="1.0"?><opml version="2.0"><body><outline text="Test" xmlUrl="http://example.com/rss"/></body></opml>`}
	req.Header.Set("Content-Type", "application/xml")
	c, rec := newTestContext(e, req)

	err := h.Import(c)
	require.NoError(t, err)
	select {
	case <-started:
	default:
		t.Fatal("task was not synchronously published before HTTP 200")
	}
	var resp handler.ImportStartedResponse
	assertJSONResponse(t, rec, http.StatusOK, &resp)
	require.Equal(t, "started", resp.Status)
	close(release)
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("import worker did not finish")
	}
}

type blockingFollowUpOPML struct {
	followUpStarted chan struct{}
	releaseFollowUp chan struct{}
	importCancelled chan struct{}
	blockImport     bool
}

func (s *blockingFollowUpOPML) Import(ctx context.Context, _ io.Reader, _ func(service.ImportProgress)) (service.ImportResult, service.ImportFollowUp, error) {
	if s.blockImport {
		<-ctx.Done()
		close(s.importCancelled)
		return service.ImportResult{}, service.ImportFollowUp{}, ctx.Err()
	}
	return service.ImportResult{FeedsCreated: 1}, service.ImportFollowUp{FeedIDs: []int64{1}}, nil
}
func (s *blockingFollowUpOPML) RunImportFollowUp(context.Context, service.ImportFollowUp) {
	close(s.followUpStarted)
	<-s.releaseFollowUp
}
func (s *blockingFollowUpOPML) Export(context.Context) ([]byte, error) { return nil, nil }

func TestOPMLHandler_PublishedTaskSurvivesHTTPContextAndCompletesBeforeFollowUp(t *testing.T) {
	opml := &blockingFollowUpOPML{followUpStarted: make(chan struct{}), releaseFollowUp: make(chan struct{})}
	tasks := service.NewImportTaskService()
	h := handler.NewOPMLHandlerHelper(opml, tasks, writerLauncherFunc(reserveAcceptedWriter))
	e := newTestEcho()
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	req := newJSONRequest(http.MethodPost, "/opml/import", nil).WithContext(requestCtx)
	req.Body = &ioReaderCloser{s: `<?xml version="1.0"?><opml version="2.0"><body><outline text="Feed" xmlUrl="https://example.com/feed"/></body></opml>`}
	req.Header.Set("Content-Type", "application/xml")
	c, rec := newTestContext(e, req)

	require.NoError(t, h.Import(c))
	require.Equal(t, http.StatusOK, rec.Code)
	cancelRequest()
	select {
	case <-opml.followUpStarted:
	case <-time.After(time.Second):
		t.Fatal("published task was killed when HTTP response context completed")
	}
	task := tasks.Get()
	require.NotNil(t, task)
	require.Equal(t, "done", task.Status, "blocking refresh must not delay task completion")
	close(opml.releaseFollowUp)
}

func TestOPMLHandler_TaskCancellationCancelsRunningImport(t *testing.T) {
	opml := &blockingFollowUpOPML{blockImport: true, importCancelled: make(chan struct{}), followUpStarted: make(chan struct{}), releaseFollowUp: make(chan struct{})}
	tasks := service.NewImportTaskService()
	h := handler.NewOPMLHandlerHelper(opml, tasks, writerLauncherFunc(reserveAcceptedWriter))
	e := newTestEcho()
	req := newJSONRequest(http.MethodPost, "/opml/import", nil)
	req.Body = &ioReaderCloser{s: `<?xml version="1.0"?><opml version="2.0"><body><outline text="Feed" xmlUrl="https://example.com/feed"/></body></opml>`}
	req.Header.Set("Content-Type", "application/xml")
	c, _ := newTestContext(e, req)

	require.NoError(t, h.Import(c))
	require.True(t, tasks.Cancel())
	select {
	case <-opml.importCancelled:
	case <-time.After(time.Second):
		t.Fatal("task cancellation did not reach running import")
	}
	require.Equal(t, "cancelled", tasks.Get().Status)
}

func TestOPMLHandler_CancelImport_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOPML := mock.NewMockOPMLService(ctrl)
	mockTask := mock.NewMockImportTaskService(ctrl)
	h := handler.NewOPMLHandlerHelper(mockOPML, mockTask, writerLauncherFunc(reserveTestWriter))

	mockTask.EXPECT().Cancel().Return(true)

	e := newTestEcho()
	req := newJSONRequest(http.MethodDelete, "/opml/import", nil)
	c, rec := newTestContext(e, req)

	err := h.CancelImport(c)
	require.NoError(t, err)

	var resp handler.ImportCancelledResponse
	assertJSONResponse(t, rec, http.StatusOK, &resp)
	require.True(t, resp.Cancelled)
}

func TestOPMLHandler_Export_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOPML := mock.NewMockOPMLService(ctrl)
	mockTask := mock.NewMockImportTaskService(ctrl)
	h := handler.NewOPMLHandlerHelper(mockOPML, mockTask, writerLauncherFunc(reserveTestWriter))

	expectedXML := []byte(`<?xml version="1.0"?><opml><body></body></opml>`)
	mockOPML.EXPECT().
		Export(gomock.Any()).
		Return(expectedXML, nil)

	e := newTestEcho()
	req := newJSONRequest(http.MethodGet, "/opml/export", nil)
	c, rec := newTestContext(e, req)

	err := h.Export(c)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/xml", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Header().Get("Content-Disposition"), "gist.opml")
	require.Equal(t, expectedXML, rec.Body.Bytes())
}

func TestOPMLHandler_ImportStatus_Idle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOPML := mock.NewMockOPMLService(ctrl)
	mockTask := mock.NewMockImportTaskService(ctrl)
	h := handler.NewOPMLHandlerHelper(mockOPML, mockTask, writerLauncherFunc(reserveTestWriter))

	// Return nil for idle status
	mockTask.EXPECT().Get().Return(nil).AnyTimes()

	e := newTestEcho()
	req := newJSONRequest(http.MethodGet, "/opml/import/status", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)
	c, rec := newTestContext(e, req)

	// Run in goroutine because ImportStatus is a blocking SSE stream
	done := make(chan bool)
	go func() {
		h.ImportStatus(c)
		close(done)
	}()

	// Wait for the context to timeout and handler to finish
	<-done

	body := rec.Body.String()
	require.Contains(t, body, "data: {\"status\":\"idle\"}")
}

// Helper types/functions for the test
type ioReaderCloser struct {
	s string
	i int
}

func (r *ioReaderCloser) Close() error { return nil }
func (r *ioReaderCloser) Read(b []byte) (int, error) {
	if r.i >= len(r.s) {
		return 0, io.EOF
	}
	n := copy(b, r.s[r.i:])
	r.i += n
	return n, nil
}

func TestOPMLHandler_Import_AdmissionRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOPML := mock.NewMockOPMLService(ctrl)
	mockTask := mock.NewMockImportTaskService(ctrl)
	admissionErr := errors.New("admission closed")
	h := handler.NewOPMLHandlerHelper(mockOPML, mockTask, writerLauncherFunc(func(context.Context, service.WriterClass) (service.WriterReservation, error) {
		return nil, admissionErr
	}))

	e := newTestEcho()
	req := newJSONRequest(http.MethodPost, "/opml/import", nil)
	req.Body = &ioReaderCloser{s: `<?xml version="1.0"?><opml version="2.0"><body/></opml>`}
	req.Header.Set("Content-Type", "application/xml")
	c, rec := newTestContext(e, req)

	err := h.Import(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.JSONEq(t, `{"error":"internal error"}`, rec.Body.String())
}
