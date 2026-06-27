package api

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Code int         `json:"code"`
	Data interface{} `json:"data,omitempty"`
	Msg  string      `json:"msg,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, resp *Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	httpStatus := http.StatusOK
	if code >= 50000 {
		httpStatus = http.StatusInternalServerError
	} else if code >= 40000 {
		httpStatus = http.StatusBadRequest
	} else if code == 40100 {
		httpStatus = http.StatusUnauthorized
	}
	writeJSON(w, httpStatus, &Response{Code: code, Msg: msg})
}

func writeSuccess(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, &Response{Code: 0, Data: data})
}
