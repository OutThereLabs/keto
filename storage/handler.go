package storage

import (
	"context"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/ory/herodot"
	"github.com/ory/x/pagination"
)

type Handler struct {
	s Manager
	h herodot.Writer
}

func NewHandler(s Manager, h herodot.Writer) *Handler {
	return &Handler{
		s: s,
		h: h,
	}
}

type GetRequest struct {
	Collection string
	Key        string
	Value      interface{}
}

func (h *Handler) Get(factory func(context.Context, *http.Request, httprouter.Params) (*GetRequest, error)) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		ctx := r.Context()
		d, err := factory(ctx, r, ps)

		if err != nil {
			h.h.WriteError(w, r, err)
			return
		}

		if err := h.s.Get(ctx, d.Collection, d.Key, d.Value); err != nil {
			h.h.WriteError(w, r, err)
			return
		}

		h.h.Write(w, r, d.Value)
	}
}

type DeleteRequest struct {
	Collection string
	Key        string
}

func (h *Handler) Delete(factory func(context.Context, *http.Request, httprouter.Params) (*DeleteRequest, error)) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		ctx := r.Context()
		d, err := factory(ctx, r, ps)
		if err != nil {
			h.h.WriteError(w, r, err)
			return
		}

		if err := h.s.Delete(ctx, d.Collection, d.Key); err != nil {
			h.h.WriteError(w, r, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

type ListRequest struct {
	Collection string
	Value      interface{}
	FilterFunc func(*ListRequest, map[string][]string, int, int)
}

func (l *ListRequest) Filter(m map[string][]string, offset int, limit int) *ListRequest {
	if l.FilterFunc != nil {
		l.FilterFunc(l, m, offset, limit)
	}
	return l
}

func ListByQuery(l *ListRequest, m map[string][]string, offset int, limit int) {
	switch val := l.Value.(type) {
	case *Roles:
		res := make(Roles, 0)
		for _, role := range *val {
			filteredRole := role.withMembers(m["member"]).withIDs(m["id"])
			if filteredRole != nil {
				res = append(res, *filteredRole)
			}
		}
		start, end := pagination.Index(limit, offset, len(res))
		res = res[start:end]
		l.Value = &res
	case *Policies:
		res := make(Policies, 0)
		for _, policy := range *val {
			filteredPolicy := policy.withSubjects(m["subject"]).withResources(m["resource"]).withActions(m["action"]).withIDs(m["id"])
			if filteredPolicy != nil {
				res = append(res, *filteredPolicy)
			}
		}
		start, end := pagination.Index(limit, offset, len(res))
		res = res[start:end]
		l.Value = &res
	default:
		panic("storage:unable to cast list request to a known type!")
	}
}

func (h *Handler) List(factory func(context.Context, *http.Request, httprouter.Params) (*ListRequest, error)) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		isFilter := false
		queryParams := r.URL.Query()
		ctx := r.Context()
		l, err := factory(ctx, r, ps)
		if err != nil {
			h.h.WriteError(w, r, err)
			return
		}
		limit, offset := pagination.Parse(r, 100, 0, 500)
		split := strings.Split(l.Collection, "/")
		collectionType := split[len(split)-1]
		if collectionType == "policies" {
			if _, ok := queryParams["action"]; ok {
				isFilter = true
			}
			if _, ok := queryParams["subject"]; ok {
				isFilter = true
			}

			if _, ok := queryParams["resource"]; ok {
				isFilter = true
			}
			if isFilter {
				// assuming that there's no limit imposed.
				if err := h.s.ListAll(ctx, l.Collection, l.Value); err != nil {
					h.h.WriteError(w, r, err)
					return
				}
			} else {
				if err := h.s.List(ctx, l.Collection, l.Value, limit, offset); err != nil {
					h.h.WriteError(w, r, err)
					return
				}
			}
		} else if collectionType == "roles" {
			if _, ok := queryParams["member"]; ok {
				isFilter = true
			}

			if isFilter {
				if err := h.s.ListAll(ctx, l.Collection, l.Value); err != nil {
					h.h.WriteError(w, r, err)
					return
				}
			} else {
				if err := h.s.List(ctx, l.Collection, l.Value, limit, offset); err != nil {
					h.h.WriteError(w, r, err)
					return
				}
			}
		} else {
			if err := h.s.List(ctx, l.Collection, l.Value, limit, offset); err != nil {
				h.h.WriteError(w, r, err)
				return
			}

		}
		m := r.URL.Query()
		h.h.Write(w, r, l.Filter(m, offset, limit).Value)
	}
}

type UpsertRequest struct {
	Collection string
	Key        string
	Value      interface{}
}

func (h *Handler) Upsert(factory func(context.Context, *http.Request, httprouter.Params) (*UpsertRequest, error)) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		ctx := r.Context()
		u, err := factory(ctx, r, ps)
		if err != nil {
			h.h.WriteError(w, r, err)
			return
		}

		if err := h.s.Upsert(ctx, u.Collection, u.Key, u.Value); err != nil {
			h.h.WriteError(w, r, err)
			return
		}

		h.h.Write(w, r, u.Value)
	}
}
