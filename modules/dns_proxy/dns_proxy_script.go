package dns_proxy

import (
	"strings"

	"github.com/bettercap/bettercap/v2/log"
	"github.com/bettercap/bettercap/v2/session"
	"github.com/evilsocket/islazy/plugin"

	"github.com/miekg/dns"

	"github.com/robertkrimen/otto"
)

type DnsProxyScript struct {
	*plugin.Plugin

	doOnRequest  bool
	doOnResponse bool
	doOnCommand  bool
}

func LoadDnsProxyScript(path string, sess *session.Session) (err error, s *DnsProxyScript) {
	log.Debug("loading proxy script %s ...", path)

	plug, err := plugin.Load(path)
	if err != nil {
		return
	}

	// define session pointer
	if err = plug.Set("env", sess.Env.Data); err != nil {
		log.Error("Error while defining environment: %+v", err)
		return
	}

	// define addSessionEvent function
	err = plug.Set("addSessionEvent", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 2 {
			log.Error("Failed to execute 'addSessionEvent' in DNS proxy: 2 arguments required, but only %d given.", len(call.ArgumentList))
			return otto.FalseValue()
		}
		ottoTag := call.Argument(0)
		if !ottoTag.IsString() {
			log.Error("Failed to execute 'addSessionEvent' in DNS proxy: first argument must be a string.")
			return otto.FalseValue()
		}
		tag := strings.TrimSpace(ottoTag.String())
		if tag == "" {
			log.Error("Failed to execute 'addSessionEvent' in DNS proxy: tag cannot be empty.")
			return otto.FalseValue()
		}
		data := call.Argument(1)
		sess.Events.Add(tag, data)
		return otto.TrueValue()
	})
	if err != nil {
		log.Error("Error while defining addSessionEvent function: %+v", err)
		return
	}

	// run onLoad if defined
	if plug.HasFunc("onLoad") {
		if _, err = plug.Call("onLoad"); err != nil {
			log.Error("Error while executing onLoad callback: %s", "\nTraceback:\n  "+err.(*otto.Error).String())
			return
		}
	}

	s = &DnsProxyScript{
		Plugin:       plug,
		doOnRequest:  plug.HasFunc("onRequest"),
		doOnResponse: plug.HasFunc("onResponse"),
		doOnCommand:  plug.HasFunc("onCommand"),
	}
	return
}

func (s *DnsProxyScript) OnRequest(req *dns.Msg, clientIP string) (jsreq, jsres *JSQuery) {
	if s.doOnRequest {
		jsreq := NewJSQuery(req, clientIP)
		jsres := NewJSQuery(req, clientIP)

		if _, err := s.Call("onRequest", jsreq, jsres); err != nil {
			log.Error("%s", err)
			return nil, nil
		} else if jsreq.WasModified() {
			jsreq.UpdateHash()
			return jsreq, nil
		} else if jsres.WasModified() {
			jsres.UpdateHash()
			return nil, jsres
		}
	}

	return nil, nil
}

func (s *DnsProxyScript) OnResponse(req, res *dns.Msg, clientIP string) (jsreq, jsres *JSQuery) {
	if s.doOnResponse {
		jsreq := NewJSQuery(req, clientIP)
		jsres := NewJSQuery(res, clientIP)

		if _, err := s.Call("onResponse", jsreq, jsres); err != nil {
			log.Error("%s", err)
			return nil, nil
		} else if jsres.WasModified() {
			jsres.UpdateHash()
			return nil, jsres
		}
	}

	return nil, nil
}

func (s *DnsProxyScript) OnCommand(cmd string) bool {
	if s.doOnCommand {
		if ret, err := s.Call("onCommand", cmd); err != nil {
			log.Error("Error while executing onCommand callback: %+v", err)
			return false
		} else if v, ok := ret.(bool); ok {
			return v
		}
	}
	return false
}