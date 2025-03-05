// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	DeepenFrom = "deepen-from" // shallow base
	Deepen     = "deepen"      // deepen <depth>
	Have       = "have"        // local have
)

// checkDeepen: check deepen and deepen-from, if deepen-from is set, ignore deepen
func (s *Server) checkDeepen(w http.ResponseWriter, r *Request) (deepen int, deepenFrom, have plumbing.Hash, err error) {
	q := r.URL.Query()
	if s := q.Get(Have); len(s) != 0 {
		if have, err = plumbing.NewHashEx(s); err != nil {
			renderFailureFormat(w, r.Request, http.StatusBadRequest, "bad have '%s'", s)
			return
		}
	}
	if ds := q.Get(DeepenFrom); len(ds) != 0 {
		if deepenFrom, err = plumbing.NewHashEx(ds); err != nil {
			renderFailureFormat(w, r.Request, http.StatusBadRequest, "bad deepen-from '%s'", ds)
			return
		}
		deepen = -1
		return
	}
	if ds := q.Get(Deepen); len(ds) != 0 {
		if deepen, err = strconv.Atoi(ds); err != nil {
			renderFailureFormat(w, r.Request, http.StatusBadRequest, "bad deepen '%s'", ds)
			return
		}
		return
	}
	deepen = 1
	return
}

// -1 means depth is infinite
func (s *Server) checkDepth(w http.ResponseWriter, r *Request) (int, error) {
	d := r.URL.Query().Get("depth")
	if d == "" {
		return -1, nil
	}
	depth, err := strconv.Atoi(d)
	if err != nil || depth < 0 {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "bad depth value '%s'", d)
		return 0, ErrStop
	}
	return depth, nil
}

func (s *Server) FetchMetadata(w http.ResponseWriter, r *Request) {
	depth, err := s.checkDepth(w, r)
	if err != nil {
		return
	}
	deepen, have, deepenFrom, err := s.checkDeepen(w, r)
	if err != nil {
		return
	}
	rev, _ := url.PathUnescape(mux.Vars(r.Request)["revision"])
	rr, err := s.open(w, r)
	if err != nil {
		return
	}
	defer rr.Close()
	ro, err := rr.ParseRev(r.Context(), rev)
	if err != nil {
		s.renderError(w, r, err)
		return
	}
	if ro.Target == nil {
		renderFailureFormat(w, r.Request, http.StatusNotFound, "rev %s target not commit", rev)
		return
	}
	p, err := protocol.NewHttpPacker(rr.ODB(), w, r.Request, depth)
	if err != nil {
		logrus.Errorf("new packer error %v", err)
		return
	}
	defer p.Close()
	for oid, o := range ro.Objects {
		if err := p.WriteAny(r.Context(), o, oid); err != nil {
			logrus.Errorf("write objects error %v", err)
			return
		}
	}
	if err := p.WriteDeepenMetadata(r.Context(), ro.Target, deepenFrom, have, deepen); err != nil {
		logrus.Errorf("write commits error %v", err)
		return
	}
	if err := p.Done(); err != nil {
		logrus.Errorf("finish metadata error %v", err)
		return
	}
}

// GetSparseMetadata: get commit metadata sparse-tree
func (s *Server) GetSparseMetadata(w http.ResponseWriter, r *Request) {
	rev, _ := url.PathUnescape(mux.Vars(r.Request)["revision"])
	if rev == "batch" {
		// Z1 protocol hijacking: avoiding overwriting of batch metadata API
		s.BatchMetadata(w, r)
		return //
	}
	depth, err := s.checkDepth(w, r)
	if err != nil {
		return
	}
	paths, err := protocol.ReadInputPaths(r.Body)
	if err != nil {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "bad input paths: %v", err)
		return
	}

	deepen, deepenFrom, have, err := s.checkDeepen(w, r)
	if err != nil {
		return
	}
	rr, err := s.open(w, r)
	if err != nil {
		return
	}
	defer rr.Close()
	ro, err := rr.ParseRev(r.Context(), rev)
	if err != nil {
		s.renderError(w, r, err)
		return
	}
	if ro.Target == nil {
		renderFailureFormat(w, r.Request, http.StatusNotFound, "rev %s target not commit", rev)
		return
	}
	cc := ro.Target
	p, err := protocol.NewHttpPacker(rr.ODB(), w, r.Request, depth)
	if err != nil {
		logrus.Errorf("new packer error %v", err)
		return
	}
	defer p.Close()
	for oid, o := range ro.Objects {
		if err := p.WriteAny(r.Context(), o, oid); err != nil {
			logrus.Errorf("write objects error %v", err)
			return
		}
	}
	if err := p.WriteDeepenSparseMetadata(r.Context(), cc, deepenFrom, have, deepen, paths); err != nil {
		logrus.Errorf("write commits error %v", err)
		return
	}
	if err := p.Done(); err != nil {
		logrus.Errorf("finish metadata error %v", err)
		return
	}
}

func (s *Server) BatchMetadata(w http.ResponseWriter, r *Request) {
	depth, err := s.checkDepth(w, r)
	if err != nil {
		return
	}
	oids, err := protocol.ReadInputOIDs(r.Body)
	if err != nil {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "batch metadata: %v", err)
		return
	}
	rr, err := s.open(w, r)
	if err != nil {
		return
	}
	defer rr.Close()
	odb := rr.ODB()
	objects := make([]any, 0, len(oids))
	for _, oid := range oids {
		a, err := odb.Objects(r.Context(), oid)
		if err != nil {
			s.renderError(w, r, err)
			return
		}
		objects = append(objects, a)
	}
	p, err := protocol.NewHttpPacker(rr.ODB(), w, r.Request, depth)
	if err != nil {
		logrus.Errorf("new packer error %v", err)
		return
	}
	defer p.Close()
	for _, a := range objects {
		switch v := a.(type) {
		case *object.Commit:
			if err := p.WriteDeduplication(r.Context(), v, v.Hash); err != nil {
				logrus.Errorf("write commit error %v", err)
				return
			}
			if err := p.WriteTree(r.Context(), v.Tree, 0); err != nil {
				logrus.Errorf("write tree error %v", err)
				return
			}
		case *object.Tree:
			if err := p.WriteTree(r.Context(), v.Hash, 0); err != nil {
				logrus.Errorf("write tree error %v", err)
				return
			}
		case *object.Tag:
			ro, err := rr.ParseRev(r.Context(), v.Object.String())
			if err != nil {
				s.renderError(w, r, err)
				return
			}
			if err := p.WriteDeduplication(r.Context(), v, v.Hash); err != nil {
				logrus.Errorf("write fragments error %v", err)
				return
			}
			for h, o := range ro.Objects {
				if err := p.WriteDeduplication(r.Context(), o, plumbing.NewHash(h)); err != nil {
					logrus.Errorf("write fragments error %v", err)
					return
				}
			}
			target := ro.Target
			if err := p.WriteDeduplication(r.Context(), target, target.Hash); err != nil {
				logrus.Errorf("write fragments error %v", err)
				return
			}
			if err := p.WriteTree(r.Context(), target.Tree, 0); err != nil {
				logrus.Errorf("write tree error %v", err)
				return
			}
		case *object.Fragments:
			if err := p.WriteDeduplication(r.Context(), v, v.Hash); err != nil {
				logrus.Errorf("write fragments error %v", err)
				return
			}
		}
	}
	if err := p.Done(); err != nil {
		logrus.Errorf("finish metadata error %v", err)
		return
	}
}
