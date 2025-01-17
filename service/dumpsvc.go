package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mariuszjedrzejewski/iec62056/actors"
	"github.com/mariuszjedrzejewski/iec62056/model"
)

var (
	// ErrBadParameter returned for bad parameters
	ErrBadParameter = errors.New("parameter error")
)

// HTTPLocalService services requests for the local measurement cache
type HTTPLocalService struct {
	listenAddress string
	localRepo     model.MeasurementRepo
	server        *http.Server
}

type GetAllHandler struct {
	server *HTTPLocalService
}

type MeasurementsResponse struct {
	Data interface{} `json:",omitempty"`
}

type errPagination struct {
	strings.Builder
}

func (s *errPagination) Error() string {
	return "bad pagination parameters\n" + s.String()
}

type pagination struct {
	page, size int
	err        *errPagination
}

func NewPagination(r *http.Request) *pagination {
	p := new(pagination)
	p.getParams(r)
	return p
}

func (p *pagination) getParams(r *http.Request) {
	page := r.FormValue("page")
	size := r.FormValue("size")

	p.page = 0
	p.size = 0

	serr := &errPagination{}
	if len(page) != 0 {
		if v, err := strconv.Atoi(r.FormValue("page")); err != nil {
			fmt.Fprintf(serr, "\tpage parameter error: %s\n", err.Error())
		} else {
			if v < 0 {
				fmt.Fprint(serr, "\tpage parameter cannog be negative\n")
			} else {
				p.page = v
			}
		}
	}
	if len(size) > 0 {
		if v, err := strconv.Atoi(r.FormValue("size")); err != nil {
			fmt.Fprintf(serr, "\tsize parameter error: %s\n", err.Error())
		} else {
			if v < 0 {
				fmt.Fprint(serr, "\tsize parameter cannot be negative\n")
			} else {
				p.size = v
			}
		}
	}
	if p.page > 0 && p.size == 0 {
		fmt.Fprint(serr, "\tnon zero page parameter requires non zero limit\n")
	}
	if serr.Len() > 0 {
		p.err = serr
	}
}

func (p *pagination) paginate() bool {
	return p.err == nil && p.size > 0
}

func NewHttpLocalService(address string, repo model.MeasurementRepo) Service {
	sm := &http.ServeMux{}
	svc := &HTTPLocalService{
		listenAddress: address,
		localRepo:     repo,
		server: &http.Server{
			Handler: sm,
			Addr:    address,
		},
	}
	gah := &GetAllHandler{
		server: svc,
	}
	// Add handlers.
	sm.Handle("/measurements/", gah)
	return svc
}

type requestContext struct {
	first, last bool
	err         error
	pag         *pagination
}

const (
	path      = "/measurements"
	pathSlash = "/measurements/"
	firstPath = "/measurements/first"
	lastPath  = "/measurements/last"
)

func getContext(r *http.Request) *requestContext {
	// Determine first and last.
	c := &requestContext{}
	p := strings.ToLower(r.URL.Path)
	switch {
	case strings.HasPrefix(p, firstPath):
		c.first = true
		return c
	case strings.HasPrefix(p, lastPath):
		c.last = true
		return c
	case p == path, p == pathSlash:
		break
	default:
		c.err = ErrBadParameter
		return c
	}

	// Determine the pagination parameters.
	if c.err = r.ParseForm(); c.err != nil {
		return c
	}
	var pag *pagination
	if pag = NewPagination(r); pag.err != nil {
		c.err = pag.err
		return c
	}
	c.pag = pag
	return c
}

func get(a *actors.PagerActor, key string) (*MeasurementsResponse, error) {
	msm, err := a.Get(key)
	if err != nil {
		return nil, err
	}
	if msm == nil {
		err := errors.New("dumpsvc.get: unexpected nil result")
		log.Printf("Error retrieving %s element: %s", key, err.Error())
		return nil, err
	}
	return &MeasurementsResponse{
		Data: msm,
	}, nil
}

func getPage(a *actors.PagerActor, pag *pagination) (*MeasurementsResponse, error) {
	msm, err := a.GetPage(pag.page, pag.size)
	if err != nil {
		return nil, err
	}

	return &MeasurementsResponse{
		Data: msm,
	}, nil
}

func getAll(a *actors.PagerActor) (*MeasurementsResponse, error) {
	msm, err := a.GetAll()
	if err != nil {
		return nil, err
	}

	return &MeasurementsResponse{
		Data: msm,
	}, nil

}

// ServeHTTP reads all entries from the local repo and returns the JSON.
func (h *GetAllHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := getContext(r)
	if ctx.err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var a = &actors.PagerActor{
		Repo: h.server.localRepo,
	}
	var mr *MeasurementsResponse
	var err error
	switch {
	case ctx.first:
		log.Print("GetAll: getFirst")
		mr, err = get(a, model.First)
	case ctx.last:
		log.Print("GetAll: getLast")
		mr, err = get(a, model.Last)
	case ctx.pag != nil && ctx.pag.paginate():
		log.Print("GetAll: getPage")
		mr, err = getPage(a, ctx.pag)
	default:
		log.Print("GetAll: getAll")
		mr, err = getAll(a)
	}
	// Get the data.
	if err != nil {
		http.Error(w, fmt.Sprintf("internal error: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	// Content type
	w.Header().Set("Content-Type", "application/json")
	// Take the output and serialize to the writer.
	// log.Printf("Get All Handler, result struct mr: %#v", *mr)
	j, err := json.Marshal(mr)
	if err != nil {
		http.Error(w, fmt.Sprintf("internal error: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	log.Printf("writing measurements response...")
	w.Write(j)
	log.Printf("writing measurements response...done")
}

// Start starts the HTTP server on the given address and port.
func (s *HTTPLocalService) Start(ctx context.Context) error {
	var err error
	var done = make(chan struct{})
	go func() {
		err = s.server.ListenAndServe()
		close(done)
	}()
	select {
	case <-done:
		return err
	case <-time.After(time.Second):
		return nil
	}
}

func (s *HTTPLocalService) Stop(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
