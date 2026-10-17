package provider

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

// fakeGNS3 is an in-memory stand-in for a GNS3 2.2 controller, built from the
// controller API pages at https://gns3-server.readthedocs.io/en/stable/endpoints.html
// (simulated from docs, not recorded). It covers what the provider calls and
// nothing else; behaviour a real server has that the docs do not describe
// (name de-duplication, emulator property defaults) is not simulated.
type fakeGNS3 struct {
	t   *testing.T
	srv *httptest.Server

	mu        sync.Mutex
	templates map[string]map[string]any // template_id -> template, built-ins included
	projects  map[string]map[string]any
	nodes     map[string]map[string]any    // node_id -> node
	links     map[string]map[string]any    // link_id -> link
	files     map[string]map[string]string // node_id -> path -> content
	images    map[string]map[string][]byte // emulator -> filename -> content (compute local)
	uploads   int
	console   int64
	dynamips  int64 // last dynamips_id handed out

	// lockedDeletes is how many project DELETEs still fail with the 409 a
	// Windows server returns while an emulator holds a project file open.
	lockedDeletes int
}

// fakeTemplates are the built-ins. 2.2.61 keeps a built-in's type-specific
// keys under "properties" (docs/api-drift.md).
var fakeTemplates = []map[string]any{
	{"builtin": true, "category": "guest", "compute_id": nil, "default_name_format": "PC{0}", "name": "VPCS",
		"symbol": ":/symbols/vpcs_guest.svg", "template_id": "19021f99-e36f-394d-b4a1-8aaa902ab9cc", "template_type": "vpcs",
		"properties": map[string]any{"base_script_file": "vpcs_base_config.txt"}},
	{"builtin": true, "category": "switch", "compute_id": nil, "console_type": "none", "default_name_format": "Switch{0}",
		"name": "Ethernet switch", "symbol": ":/symbols/ethernet_switch.svg", "template_id": "1966b864-93e7-32d5-965f-001384eec461",
		"template_type": "ethernet_switch"},
	{"builtin": true, "category": "switch", "compute_id": nil, "default_name_format": "Hub{0}", "name": "Ethernet hub",
		"symbol": ":/symbols/hub.svg", "template_id": "b4503ea9-d6b6-3695-9fe4-1db3b39290b0", "template_type": "ethernet_hub"},
}

var fakeCompute = map[string]any{
	"compute_id": "local", "name": "fake-local", "host": "127.0.0.1", "port": 3080, "protocol": "http",
	"connected": true, "user": nil,
}

func newFakeGNS3(t *testing.T) *fakeGNS3 {
	t.Helper()
	f := &fakeGNS3{
		t: t, projects: map[string]map[string]any{}, nodes: map[string]map[string]any{},
		links: map[string]map[string]any{}, files: map[string]map[string]string{}, console: 5000,
		images: map[string]map[string][]byte{"qemu": {}, "dynamips": {}}, templates: map[string]map[string]any{},
	}
	for _, tpl := range fakeTemplates {
		f.templates[tpl["template_id"].(string)] = maps.Clone(tpl)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v2/version", func(w http.ResponseWriter, _ *http.Request) {
		reply(w, 200, map[string]any{"version": "2.2.52", "local": true})
	})
	mux.HandleFunc("GET /v2/computes", func(w http.ResponseWriter, _ *http.Request) { reply(w, 200, []any{fakeCompute}) })
	mux.HandleFunc("GET /v2/computes/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "local" {
			fail(w, 404, "Compute ID %s doesn't exist", r.PathValue("id"))
			return
		}
		reply(w, 200, fakeCompute)
	})
	mux.HandleFunc("GET /v2/computes/{id}/{emulator}/images", f.withImages(f.listImages))
	mux.HandleFunc("POST /v2/computes/{id}/{emulator}/images/{filename...}", f.withImages(func(w http.ResponseWriter, r *http.Request, imgs map[string][]byte) {
		body, _ := io.ReadAll(r.Body)
		imgs[r.PathValue("filename")] = body
		f.uploads++
		reply(w, 200, map[string]any{})
	}))
	mux.HandleFunc("GET /v2/templates", func(w http.ResponseWriter, _ *http.Request) {
		out := make([]any, 0, len(f.templates))
		for _, id := range slices.Sorted(maps.Keys(f.templates)) {
			out = append(out, f.templates[id])
		}
		reply(w, 200, out)
	})
	mux.HandleFunc("POST /v2/templates", f.createTemplate)
	mux.HandleFunc("GET /v2/templates/{id}", f.withTemplate(func(w http.ResponseWriter, _ *http.Request, tpl map[string]any) { reply(w, 200, tpl) }))
	mux.HandleFunc("PUT /v2/templates/{id}", f.withTemplate(f.updateTemplate))
	mux.HandleFunc("DELETE /v2/templates/{id}", f.withTemplate(func(w http.ResponseWriter, _ *http.Request, tpl map[string]any) {
		if tpl["builtin"] == true {
			fail(w, 409, "Template ID %s cannot be deleted because it is a builtin", tpl["template_id"])
			return
		}
		delete(f.templates, tpl["template_id"].(string))
		w.WriteHeader(204)
	}))
	mux.HandleFunc("GET /v2/projects", f.listProjects)
	mux.HandleFunc("POST /v2/projects", f.createProject)
	mux.HandleFunc("GET /v2/projects/{pid}", f.withProject(func(w http.ResponseWriter, _ *http.Request, p map[string]any) { reply(w, 200, p) }))
	mux.HandleFunc("PUT /v2/projects/{pid}", f.withProject(f.updateProject))
	mux.HandleFunc("DELETE /v2/projects/{pid}", f.withProject(f.deleteProject))
	mux.HandleFunc("POST /v2/projects/{pid}/open", f.withProject(func(w http.ResponseWriter, _ *http.Request, p map[string]any) {
		p["status"] = "opened"
		reply(w, 201, p)
	}))
	mux.HandleFunc("POST /v2/projects/{pid}/close", f.withProject(func(w http.ResponseWriter, _ *http.Request, p map[string]any) {
		p["status"] = "closed"
		w.WriteHeader(204)
	}))
	mux.HandleFunc("POST /v2/projects/{pid}/nodes", f.withOpenProject(f.createNode))
	mux.HandleFunc("POST /v2/projects/{pid}/templates/{tid}", f.withOpenProject(f.createNodeFromTemplate))
	mux.HandleFunc("GET /v2/projects/{pid}/nodes/{nid}", f.withNode(func(w http.ResponseWriter, _ *http.Request, n map[string]any) { reply(w, 200, n) }))
	mux.HandleFunc("PUT /v2/projects/{pid}/nodes/{nid}", f.withNode(f.updateNode))
	mux.HandleFunc("DELETE /v2/projects/{pid}/nodes/{nid}", f.withNode(f.deleteNode))
	mux.HandleFunc("POST /v2/projects/{pid}/nodes/{nid}/start", f.withNode(func(w http.ResponseWriter, _ *http.Request, n map[string]any) {
		n["status"] = "started"
		reply(w, 200, n)
	}))
	mux.HandleFunc("POST /v2/projects/{pid}/nodes/{nid}/stop", f.withNode(func(w http.ResponseWriter, _ *http.Request, n map[string]any) {
		n["status"] = "stopped"
		reply(w, 200, n)
	}))
	mux.HandleFunc("GET /v2/projects/{pid}/nodes/{nid}/files/{path...}", f.withNode(func(w http.ResponseWriter, r *http.Request, n map[string]any) {
		content, ok := f.files[n["node_id"].(string)][r.PathValue("path")]
		if !ok {
			fail(w, 404, "GET /projects/%s/files/project-files/%s/%s/%s not found", n["project_id"], n["node_type"], n["node_id"], r.PathValue("path"))
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte(content))
	}))
	mux.HandleFunc("POST /v2/projects/{pid}/nodes/{nid}/files/{path...}", f.withNode(func(w http.ResponseWriter, r *http.Request, n map[string]any) {
		body, _ := io.ReadAll(r.Body)
		f.writeFile(n["node_id"].(string), r.PathValue("path"), string(body))
		w.WriteHeader(201)
	}))
	mux.HandleFunc("POST /v2/projects/{pid}/links", f.withOpenProject(f.createLink))
	mux.HandleFunc("GET /v2/projects/{pid}/links/{lid}", f.withLink(func(w http.ResponseWriter, _ *http.Request, l map[string]any) { reply(w, 200, l) }))
	mux.HandleFunc("PUT /v2/projects/{pid}/links/{lid}", f.withLink(f.updateLink))
	mux.HandleFunc("DELETE /v2/projects/{pid}/links/{lid}", f.withLink(func(w http.ResponseWriter, _ *http.Request, l map[string]any) {
		delete(f.links, l["link_id"].(string))
		w.WriteHeader(204)
	}))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("fake GNS3: unhandled %s %s", r.Method, r.URL.Path)
		fail(w, 404, "no route for %s %s", r.Method, r.URL.Path)
	})
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// providerBlock points the provider at the fake server.
func (f *fakeGNS3) providerBlock() string {
	u, _ := url.Parse(f.srv.URL)
	return fmt.Sprintf("provider \"gns3\" {\n  host = %q\n  port = %s\n}\n", u.Hostname(), u.Port())
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func fail(w http.ResponseWriter, status int, format string, args ...any) {
	reply(w, status, map[string]any{"message": fmt.Sprintf(format, args...), "status": status})
}

func decode(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, 400, "Invalid JSON: %v", err)
		return nil, false
	}
	return body, true
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// templateDefaults are the type-specific keys and defaults a 2.2.61 POST
// /v2/templates fills in for a minimal template (seen on 2.2.61,
// docs/api-drift.md), next to its own category, symbol and
// default_name_format. A create with any other key is 400, and a value whose
// JSON type differs from the default's is 400. Types not listed take any key.
var templateDefaults = map[string]map[string]any{
	"vpcs": {
		"category": "guest", "symbol": ":/symbols/vpcs_guest.svg", "default_name_format": "PC{0}",
		"base_script_file": "vpcs_base_config.txt", "console_auto_start": false, "console_type": "telnet",
	},
	"qemu": {
		"category": "guest", "symbol": ":/symbols/qemu_guest.svg", "default_name_format": "{name}-{0}",
		"adapter_type": "e1000", "adapters": 1, "bios_image": "", "boot_priority": "c", "cdrom_image": "",
		"console_auto_start": false, "console_type": "telnet", "cpu_throttling": 0, "cpus": 1, "create_config_disk": false,
		"custom_adapters": []any{}, "first_port_name": "", "hda_disk_image": "", "hda_disk_interface": "none",
		"hdb_disk_image": "", "hdb_disk_interface": "none", "hdc_disk_image": "", "hdc_disk_interface": "none",
		"hdd_disk_image": "", "hdd_disk_interface": "none", "initrd": "", "kernel_command_line": "", "kernel_image": "",
		"legacy_networking": false, "linked_clone": true, "mac_address": "", "on_close": "power_off", "options": "",
		"platform": "", "port_name_format": "Ethernet{0}", "port_segment_size": 0, "process_priority": "normal",
		"qemu_path": "", "ram": 256, "replicate_network_connection_state": true, "tpm": false, "uefi": false, "usage": "",
	},
	"dynamips": {
		"category": "router", "symbol": ":/symbols/router.svg", "default_name_format": "R{0}",
		"auto_delete_disks": false, "console_auto_start": false, "console_type": "telnet", "disk0": 0, "disk1": 0,
		"exec_area": 64, "idlemax": 500, "idlepc": "", "idlesleep": 30, "image": "", "mac_addr": "", "midplane": "vxr",
		"mmap": true, "npe": "npe-400", "nvram": 512, "platform": "", "private_config": "", "ram": 512, "sparsemem": true,
		"startup_config": "ios_base_startup-config.txt", "system_id": "FTX0945W0MY", "usage": "",
	},
	"docker": {
		"category": "guest", "symbol": ":/symbols/docker_guest.svg", "default_name_format": "{name}-{0}",
		"adapters": 1, "console_auto_start": false, "console_http_path": "/", "console_http_port": 80,
		"console_resolution": "1024x768", "console_type": "telnet", "custom_adapters": []any{}, "environment": "",
		"extra_hosts": "", "extra_volumes": []any{}, "image": "", "mac_address": "", "start_command": "", "usage": "",
	},
}

// templateCategories is the category enum (a 2.2.61 create or update with any
// other value is 400).
var templateCategories = []string{"router", "switch", "guest", "firewall"}

func (f *fakeGNS3) findTemplate(id string) map[string]any { return f.templates[id] }

func (f *fakeGNS3) withTemplate(h func(http.ResponseWriter, *http.Request, map[string]any)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tpl := f.findTemplate(r.PathValue("id"))
		if tpl == nil {
			fail(w, 404, "Template ID %s doesn't exist", r.PathValue("id"))
			return
		}
		h(w, r, tpl)
	}
}

func (f *fakeGNS3) createTemplate(w http.ResponseWriter, r *http.Request) {
	body, ok := decode(w, r)
	if !ok {
		return
	}
	for _, k := range []string{"compute_id", "name", "template_type"} {
		if _, ok := body[k]; !ok {
			fail(w, 400, "JSON schema error with API request '/v2/templates': '%s' is a required property", k)
			return
		}
	}
	tplType, _ := body["template_type"].(string)
	if !slices.Contains(templateTypes, tplType) {
		fail(w, 400, "JSON schema error with API request '/v2/templates': '%s' is not one of %v", tplType, templateTypes)
		return
	}
	if c, ok := body["category"].(string); ok && !slices.Contains(templateCategories, c) {
		fail(w, 400, "JSON schema error with API request '/v2/templates': '%s' is not valid under any of the given schemas", c)
		return
	}
	for _, k := range requiredTemplateProperties[tplType] {
		if _, ok := body[k]; !ok {
			fail(w, 400, "JSON schema error adding template: '%s' is a required property", k)
			return
		}
	}
	tpl := map[string]any{}
	defaults, typed := templateDefaults[tplType]
	maps.Copy(tpl, defaults)
	for k, v := range body {
		if typed && !slices.Contains(gns3.TemplateCommonKeys, k) {
			def, known := defaults[k]
			if !known {
				fail(w, 400, "JSON schema error adding template: Additional properties are not allowed ('%s' was unexpected)", k)
				return
			}
			if fmt.Sprintf("%T", def) != fmt.Sprintf("%T", normalizeJSON(v)) {
				fail(w, 400, "JSON schema error adding template: %v is not of type '%T'", v, def)
				return
			}
		}
		tpl[k] = v
	}
	tpl["template_id"], tpl["builtin"] = newUUID(), false
	f.templates[tpl["template_id"].(string)] = tpl
	reply(w, 201, tpl)
}

// normalizeJSON gives a decoded JSON number the Go type the defaults use.
func normalizeJSON(v any) any {
	if n, ok := v.(float64); ok && n == float64(int(n)) {
		return int(n)
	}
	return v
}

// updateTemplate follows 2.2.61: only name and category are checked, every
// other key is stored as sent, and template_type, compute_id, template_id and
// builtin are ignored (docs/api-drift.md).
func (f *fakeGNS3) updateTemplate(w http.ResponseWriter, r *http.Request, tpl map[string]any) {
	body, ok := decode(w, r)
	if !ok {
		return
	}
	if v, ok := body["name"]; ok {
		if s, isString := v.(string); !isString || s == "" {
			fail(w, 400, "JSON schema error with API request: %v is too short or not of type 'string'", v)
			return
		}
	}
	if c, ok := body["category"].(string); ok && !slices.Contains(templateCategories, c) {
		fail(w, 400, "JSON schema error with API request: '%s' is not valid under any of the given schemas", c)
		return
	}
	for k, v := range body {
		switch k {
		case "template_type", "compute_id", "template_id", "builtin":
			continue
		}
		tpl[k] = v
	}
	reply(w, 200, tpl)
}

func (f *fakeGNS3) listProjects(w http.ResponseWriter, _ *http.Request) {
	out := make([]any, 0, len(f.projects))
	for _, p := range f.projects {
		out = append(out, p)
	}
	reply(w, 200, out)
}

func (f *fakeGNS3) createProject(w http.ResponseWriter, r *http.Request) {
	body, ok := decode(w, r)
	if !ok {
		return
	}
	name, _ := body["name"].(string)
	if name == "" {
		fail(w, 400, "name is required")
		return
	}
	for _, p := range f.projects {
		if p["name"] == name {
			fail(w, 409, "Project '%s' already exists", name)
			return
		}
	}
	id := newUUID()
	p := map[string]any{
		"project_id": id, "name": name, "path": "/opt/gns3/projects/" + id, "filename": name + ".gns3",
		"status": "opened", "auto_start": false, "auto_open": false, "auto_close": true,
		"scene_width": 2000, "scene_height": 1000, "grid_size": 75, "show_grid": false,
	}
	for _, k := range []string{"path", "auto_start", "auto_open", "auto_close", "scene_width", "scene_height", "grid_size", "show_grid"} {
		if v, ok := body[k]; ok {
			p[k] = v
		}
	}
	f.projects[id] = p
	reply(w, 201, p)
}

func (f *fakeGNS3) withProject(h func(http.ResponseWriter, *http.Request, map[string]any)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := f.projects[r.PathValue("pid")]
		if !ok {
			fail(w, 404, "Project ID %s doesn't exist", r.PathValue("pid"))
			return
		}
		h(w, r, p)
	}
}

func (f *fakeGNS3) withOpenProject(h func(http.ResponseWriter, *http.Request, map[string]any)) http.HandlerFunc {
	return f.withProject(func(w http.ResponseWriter, r *http.Request, p map[string]any) {
		if p["status"] != "opened" {
			fail(w, 409, "Project %s is closed", p["project_id"])
			return
		}
		h(w, r, p)
	})
}

func (f *fakeGNS3) updateProject(w http.ResponseWriter, r *http.Request, p map[string]any) {
	body, ok := decode(w, r)
	if !ok {
		return
	}
	for k, v := range body {
		p[k] = v
	}
	reply(w, 200, p)
}

func (f *fakeGNS3) deleteProject(w http.ResponseWriter, _ *http.Request, p map[string]any) {
	id := p["project_id"].(string)
	if f.lockedDeletes > 0 {
		f.lockedDeletes--
		fail(w, 409, "Cannot delete project directory %s: [WinError 32] The process cannot access the file because it is being used by another process", p["path"])
		return
	}
	for nid, n := range f.nodes {
		if n["project_id"] == id {
			delete(f.nodes, nid)
		}
	}
	for lid, l := range f.links {
		if l["project_id"] == id {
			delete(f.links, lid)
		}
	}
	delete(f.projects, id)
	w.WriteHeader(204)
}

// newNode builds a node the way the docs sample shows it, with ports per
// node type: vpcs has Ethernet0; switches and hubs have Ethernet0..7.
func (f *fakeGNS3) newNode(projectID, nodeType, name, computeID string, templateID any) map[string]any {
	var ports []any
	count := 1
	symbol := ":/symbols/computer.svg"
	consoleType := any("telnet")
	console := any(f.console)
	switch nodeType {
	case "ethernet_switch":
		count, symbol, consoleType, console = 8, ":/symbols/ethernet_switch.svg", "none", nil
	case "ethernet_hub":
		count, symbol, consoleType, console = 8, ":/symbols/hub.svg", "none", nil
	}
	if console != nil {
		f.console++
	}
	for i := 0; i < count; i++ {
		ports = append(ports, map[string]any{
			"adapter_number": 0, "port_number": i, "name": fmt.Sprintf("Ethernet%d", i),
			"short_name": fmt.Sprintf("e%d", i), "link_type": "ethernet",
			"data_link_types": map[string]any{"Ethernet": "DLT_EN10MB"},
		})
	}
	return map[string]any{
		"node_id": newUUID(), "project_id": projectID, "compute_id": computeID, "template_id": templateID,
		"node_type": nodeType, "name": name, "status": "stopped", "symbol": symbol,
		"console": console, "console_type": consoleType, "console_host": "127.0.0.1",
		"x": 0, "y": 0, "z": 1, "ports": ports, "properties": f.newProperties(nodeType),
	}
}

// newProperties: 2.2.61 numbers dynamips routers per project, from 1, in
// properties.dynamips_id; other types start empty here.
func (f *fakeGNS3) newProperties(nodeType string) map[string]any {
	if nodeType != "dynamips" {
		return map[string]any{}
	}
	f.dynamips++
	return map[string]any{"dynamips_id": f.dynamips}
}

// dynamipsConfigs are the properties a 2.2.61 dynamips compute writes to
// configs/i<dynamips_id>_<name>.cfg at create, with "!\n" in front, and that
// its update schema rejects (seen on 2.2.61, docs/api-drift.md).
var dynamipsConfigs = map[string]string{
	"startup_config_content": "startup-config", "private_config_content": "private-config",
}

func (f *fakeGNS3) createNode(w http.ResponseWriter, r *http.Request, p map[string]any) {
	body, ok := decode(w, r)
	if !ok {
		return
	}
	nodeType, _ := body["node_type"].(string)
	name, _ := body["name"].(string)
	computeID, _ := body["compute_id"].(string)
	if nodeType == "" || name == "" || computeID == "" {
		fail(w, 400, "compute_id, name and node_type are required")
		return
	}
	n := f.newNode(p["project_id"].(string), nodeType, name, computeID, nil)
	for _, k := range []string{"x", "y", "z", "symbol", "console_type"} {
		if v, ok := body[k]; ok {
			n[k] = v
		}
	}
	mergeProperties(n, body)
	// 2.2.61 writes a VPCS startup_script given at create to startup.vpc.
	if s, ok := n["properties"].(map[string]any)["startup_script"].(string); ok && nodeType == "vpcs" {
		f.writeFile(n["node_id"].(string), "startup.vpc", s)
	}
	if nodeType == "dynamips" {
		props := n["properties"].(map[string]any)
		for k, name := range dynamipsConfigs {
			if s, ok := props[k].(string); ok {
				f.writeFile(n["node_id"].(string), fmt.Sprintf("configs/i%d_%s.cfg", props["dynamips_id"], name), "!\n"+s)
			}
		}
	}
	f.nodes[n["node_id"].(string)] = n
	reply(w, 201, n)
}

func (f *fakeGNS3) createNodeFromTemplate(w http.ResponseWriter, r *http.Request, p map[string]any) {
	tpl := f.findTemplate(r.PathValue("tid"))
	if tpl == nil {
		fail(w, 404, "Template ID %s doesn't exist", r.PathValue("tid"))
		return
	}
	// The controller sends the template's keys to the compute, whose vpcs
	// create schema rejects any it does not know (seen on 2.2.61 after a PUT
	// stored ram and bogus_prop on a vpcs template, docs/api-drift.md).
	if tpl["template_type"] == "vpcs" && tpl["builtin"] != true {
		for k := range tpl {
			if _, known := templateDefaults["vpcs"][k]; !known && !slices.Contains(gns3.TemplateCommonKeys, k) {
				fail(w, 400, "Bad request .../vpcs/nodes: Additional properties are not allowed ('%s' was unexpected)", k)
				return
			}
		}
	}
	body, ok := decode(w, r)
	if !ok {
		return
	}
	if _, ok := body["x"]; !ok {
		fail(w, 400, "x is a required property")
		return
	}
	if _, ok := body["y"]; !ok {
		fail(w, 400, "y is a required property")
		return
	}
	computeID, _ := tpl["compute_id"].(string)
	if computeID == "" {
		computeID, _ = body["compute_id"].(string)
	}
	if computeID == "" {
		fail(w, 400, "Template %s has no compute and no compute_id was given", tpl["name"])
		return
	}
	name, _ := body["name"].(string)
	if name == "" {
		name = strings.ReplaceAll(tpl["default_name_format"].(string), "{0}", "1")
	}
	n := f.newNode(p["project_id"].(string), tpl["template_type"].(string), name, computeID, tpl["template_id"])
	n["symbol"] = tpl["symbol"]
	n["x"], n["y"] = body["x"], body["y"]
	f.nodes[n["node_id"].(string)] = n
	reply(w, 201, n)
}

func mergeProperties(n, body map[string]any) {
	props, _ := body["properties"].(map[string]any)
	cur := n["properties"].(map[string]any)
	for k, v := range props {
		cur[k] = v
	}
}

func (f *fakeGNS3) withNode(h func(http.ResponseWriter, *http.Request, map[string]any)) http.HandlerFunc {
	return f.withProject(func(w http.ResponseWriter, r *http.Request, p map[string]any) {
		n, ok := f.nodes[r.PathValue("nid")]
		if !ok || n["project_id"] != p["project_id"] || p["status"] != "opened" {
			fail(w, 404, "Node ID %s doesn't exist", r.PathValue("nid"))
			return
		}
		h(w, r, n)
	})
}

func (f *fakeGNS3) updateNode(w http.ResponseWriter, r *http.Request, n map[string]any) {
	body, ok := decode(w, r)
	if !ok {
		return
	}
	// The controller forwards its cached properties plus the new ones to the
	// compute, whose VPCS update schema has no startup_script (seen on 2.2.61).
	if n["node_type"] == "vpcs" {
		props, _ := body["properties"].(map[string]any)
		_, cached := n["properties"].(map[string]any)["startup_script"]
		if _, sent := props["startup_script"]; sent || cached {
			fail(w, 400, "JSON schema error with API request: Additional properties are not allowed ('startup_script' was unexpected)")
			return
		}
	}
	if n["node_type"] == "dynamips" {
		props, _ := body["properties"].(map[string]any)
		for k := range dynamipsConfigs {
			_, cached := n["properties"].(map[string]any)[k]
			if _, sent := props[k]; sent || cached {
				fail(w, 400, "JSON schema error with API request: Additional properties are not allowed ('%s' was unexpected)", k)
				return
			}
		}
	}
	for _, k := range []string{"name", "x", "y", "z", "symbol", "console_type"} {
		if v, ok := body[k]; ok {
			n[k] = v
		}
	}
	mergeProperties(n, body)
	reply(w, 200, n)
}

func (f *fakeGNS3) deleteNode(w http.ResponseWriter, _ *http.Request, n map[string]any) {
	id := n["node_id"].(string)
	for lid, l := range f.links {
		for _, e := range l["nodes"].([]any) {
			if e.(map[string]any)["node_id"] == id {
				delete(f.links, lid)
			}
		}
	}
	delete(f.nodes, id)
	delete(f.files, id)
	w.WriteHeader(204)
}

func (f *fakeGNS3) createLink(w http.ResponseWriter, r *http.Request, p map[string]any) {
	body, ok := decode(w, r)
	if !ok {
		return
	}
	ends, _ := body["nodes"].([]any)
	if len(ends) != 2 {
		fail(w, 400, "A link needs two nodes")
		return
	}
	var ids []string
	for _, e := range ends {
		end := e.(map[string]any)
		nid, _ := end["node_id"].(string)
		n, ok := f.nodes[nid]
		if !ok || n["project_id"] != p["project_id"] {
			fail(w, 404, "Node ID %s doesn't exist", nid)
			return
		}
		if !hasPort(n, end["adapter_number"], end["port_number"]) {
			fail(w, 409, "Port %v/%v is not found on %s", end["adapter_number"], end["port_number"], n["name"])
			return
		}
		for _, l := range f.links {
			for _, le := range l["nodes"].([]any) {
				o := le.(map[string]any)
				if o["node_id"] == nid && o["adapter_number"] == end["adapter_number"] && o["port_number"] == end["port_number"] {
					fail(w, 409, "Port %v/%v is already used on %s", end["adapter_number"], end["port_number"], n["name"])
					return
				}
			}
		}
		ids = append(ids, nid)
	}
	if ids[0] == ids[1] {
		fail(w, 409, "Cannot connect to itself")
		return
	}
	filters := body["filters"]
	if filters == nil {
		filters = map[string]any{}
	}
	suspend, _ := body["suspend"].(bool)
	l := map[string]any{
		"link_id": newUUID(), "project_id": p["project_id"], "link_type": "ethernet", "nodes": ends,
		"filters": filters, "suspend": suspend, "capturing": false,
	}
	f.links[l["link_id"].(string)] = l
	reply(w, 201, l)
}

func hasPort(n map[string]any, adapter, port any) bool {
	for _, p := range n["ports"].([]any) {
		pm := p.(map[string]any)
		if fmt.Sprint(pm["adapter_number"]) == fmt.Sprint(adapter) && fmt.Sprint(pm["port_number"]) == fmt.Sprint(port) {
			return true
		}
	}
	return false
}

func (f *fakeGNS3) withLink(h func(http.ResponseWriter, *http.Request, map[string]any)) http.HandlerFunc {
	return f.withProject(func(w http.ResponseWriter, r *http.Request, p map[string]any) {
		l, ok := f.links[r.PathValue("lid")]
		if !ok || l["project_id"] != p["project_id"] || p["status"] != "opened" {
			fail(w, 404, "Link ID %s doesn't exist", r.PathValue("lid"))
			return
		}
		h(w, r, l)
	})
}

func (f *fakeGNS3) updateLink(w http.ResponseWriter, r *http.Request, l map[string]any) {
	body, ok := decode(w, r)
	if !ok {
		return
	}
	if v, ok := body["filters"]; ok {
		l["filters"] = v
	}
	if v, ok := body["suspend"]; ok {
		l["suspend"] = v
	}
	reply(w, 201, l)
}

func (f *fakeGNS3) writeFile(nodeID, path, content string) {
	if f.files[nodeID] == nil {
		f.files[nodeID] = map[string]string{}
	}
	f.files[nodeID][path] = content
}

// withImages resolves the compute and emulator the way 2.2.61 on Windows
// does: only "local" exists, and it serves qemu and dynamips images but has
// no IOU or Docker (404 "GET /iou/images not found", docs/api-drift.md).
func (f *fakeGNS3) withImages(h func(http.ResponseWriter, *http.Request, map[string][]byte)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "local" {
			fail(w, 404, "Compute ID %s doesn't exist", r.PathValue("id"))
			return
		}
		imgs, ok := f.images[r.PathValue("emulator")]
		if !ok {
			rest := strings.TrimPrefix(r.URL.Path, "/v2/computes/local")
			fail(w, 404, "%s %s not found", r.Method, rest)
			return
		}
		h(w, r, imgs)
	}
}

// listImages lists the top-level files the way list_images in
// gns3server/utils/images.py (2.2 branch) does: files under 7 bytes are left
// out, dynamips keeps only 32-bit big-endian ELF files and qemu drops ELF files.
func (f *fakeGNS3) listImages(w http.ResponseWriter, r *http.Request, imgs map[string][]byte) {
	out := []any{}
	for _, name := range slices.Sorted(maps.Keys(imgs)) {
		content := imgs[name]
		if len(content) < 7 {
			continue
		}
		isELF := bytes.HasPrefix(content, elfMagic)
		if r.PathValue("emulator") == "dynamips" && !bytes.HasPrefix(content, dynamipsELF) ||
			r.PathValue("emulator") == "qemu" && isELF {
			continue
		}
		sum := md5.Sum(content)
		out = append(out, map[string]any{
			"filename": name, "path": name, "md5sum": hex.EncodeToString(sum[:]), "filesize": len(content),
		})
	}
	reply(w, 200, out)
}

// putImage places a file on the local compute without an upload, as a file
// copied there by hand would be.
func (f *fakeGNS3) putImage(emulator, filename string, content []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.images[emulator][filename] = content
}

// removeImage deletes a file from the local compute behind the provider's back.
func (f *fakeGNS3) removeImage(emulator, filename string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.images[emulator], filename)
}

// imageUploads is how many image uploads the fake has received.
func (f *fakeGNS3) imageUploads() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.uploads
}

// lockProjectDeletes makes the next n project DELETEs fail as a locked file would.
func (f *fakeGNS3) lockProjectDeletes(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lockedDeletes = n
}

// counts reports how many projects, nodes and links the fake holds.
func (f *fakeGNS3) counts() (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.projects), len(f.nodes), len(f.links)
}

// closeProject closes a project the way a server restart would.
func (f *fakeGNS3) closeProject(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projects[id]["status"] = "closed"
}

// elfMagic starts every ELF file; dynamipsELF is the 32-bit big-endian ELF
// header list_images requires for dynamips (gns3server/utils/images.py, 2.2).
var (
	elfMagic    = []byte{0x7f, 'E', 'L', 'F'}
	dynamipsELF = []byte{0x7f, 'E', 'L', 'F', 1, 2, 1}
)
