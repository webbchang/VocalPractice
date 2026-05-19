package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type FileEntry struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Lines     int    `json:"lines"`
	Extension string `json:"extension"`
	IsDir     bool   `json:"is_dir"`
}

type DirTree struct {
	Name     string     `json:"name"`
	IsDir    bool       `json:"is_dir"`
	Children []*DirTree `json:"children,omitempty"`
}

type SnapshotData struct {
	Timestamp    string            `json:"timestamp"`
	ProjectName  string            `json:"project_name"`
	FileCount    int               `json:"file_count"`
	DirCount     int               `json:"dir_count"`
	TotalLines   int               `json:"total_lines"`
	TotalSize    int64             `json:"total_size"`
	FilesByExt   map[string]int    `json:"files_by_ext"`
	LinesByExt   map[string]int    `json:"lines_by_ext"`
	TopFiles     []FileEntry       `json:"top_files"`
	DirTreeJSON  string            `json:"dir_tree_json"`
	APIRoutes    []string          `json:"api_routes"`
	DomainModels []DomainModelInfo `json:"domain_models"`
	GoModules    []string          `json:"go_modules"`
	GitStatus    string            `json:"git_status"`
	GitLog       string            `json:"git_log"`
	TestResult   string            `json:"test_result"`
}

type DomainModelInfo struct {
	Package string   `json:"package"`
	File    string   `json:"file"`
	Types   []string `json:"types"`
	Fields  int      `json:"fields"`
}

var tmpl *template.Template

func init() {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"divf": func(a, b int64) float64 {
			if b == 0 {
				return 0
			}
			return float64(a) / float64(b)
		},
		"percentF": func(part, total int) float64 {
			if total == 0 {
				return 0
			}
			return float64(part) / float64(total) * 100
		},
		"badgeClass": func(ext string) string {
			switch ext {
			case ".go":
				return "badge-go"
			case ".js":
				return "badge-js"
			case ".css":
				return "badge-css"
			case ".html":
				return "badge-html"
			case ".md":
				return "badge-md"
			case ".json":
				return "badge-json"
			case ".bak":
				return "badge-other"
			default:
				return "badge-other"
			}
		},
		"gitCommitCount": func(log string) string {
			s := strings.TrimSpace(log)
			if s == "" || s == "(error: exit status 128)" {
				return "0"
			}
			lines := strings.Split(s, "\n")
			return fmt.Sprintf("%d", len(lines))
		},
		"renderTree": func(jsonStr string) template.HTML {
			var root DirTree
			json.Unmarshal([]byte(jsonStr), &root)
			return template.HTML(renderDirTreeHTML(&root, 0))
		},
	}
	tmpl = template.Must(template.New("snapshot").Funcs(funcMap).Parse(snapshotTemplate))
}

func main() {
	projectRoot := "."
	if len(os.Args) > 1 {
		projectRoot = os.Args[1]
	}

	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve path: %v\n", err)
		os.Exit(1)
	}

	data := SnapshotData{
		Timestamp:   time.Now().Format(time.RFC3339),
		ProjectName: filepath.Base(absRoot),
		FilesByExt:  make(map[string]int),
		LinesByExt:  make(map[string]int),
	}

	var allFiles []FileEntry
	excludeDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"uploads":      true,
		"output":       true,
		"test_data":    true,
		"snapshot":     true,
	}
	excludeExt := map[string]bool{
		".exe": true,
		".bin": true,
		".mid": true,
		".wav": true,
		".bak": true,
	}

	root := &DirTree{Name: filepath.Base(absRoot), IsDir: true}

	err = filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(absRoot, path)
		if rel == "." {
			return nil
		}

		parts := strings.Split(rel, string(filepath.Separator))
		for _, p := range parts {
			if excludeDirs[p] {
				return filepath.SkipDir
			}
		}

		insertIntoTree(root, rel, info.IsDir())

		if info.IsDir() {
			data.DirCount++
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if excludeExt[ext] {
			return nil
		}

		lines := countLines(path)
		fe := FileEntry{
			Path:      rel,
			Size:      info.Size(),
			Lines:     lines,
			Extension: ext,
		}
		allFiles = append(allFiles, fe)

		data.FileCount++
		data.TotalSize += info.Size()
		data.TotalLines += lines
		data.FilesByExt[ext]++
		data.LinesByExt[ext] += lines

		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk error: %v\n", err)
	}

	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].Lines > allFiles[j].Lines
	})
	if len(allFiles) > 30 {
		data.TopFiles = allFiles[:30]
	} else {
		data.TopFiles = allFiles
	}

	treeJSON, _ := json.MarshalIndent(root, "", "  ")
	data.DirTreeJSON = string(treeJSON)
	data.APIRoutes = extractAPIRoutes(filepath.Join(absRoot, "cmd", "server", "main.go"))
	data.DomainModels = extractDomainModels(filepath.Join(absRoot, "internal", "domain"))
	data.GoModules = extractGoModules(filepath.Join(absRoot, "go.mod"))
	data.GitStatus = runCmd("git", "status", "--short")
	data.GitLog = runCmd("git", "log", "--oneline", "-20")
	data.TestResult = runCmd("go", "test", "./...", "-count=1", "-timeout=30s")

	outputPath := filepath.Join(absRoot, "snapshot", "snapshot.html")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create snapshot dir: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create output: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		fmt.Fprintf(os.Stderr, "template error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Snapshot saved to: %s\n", outputPath)
	fmt.Printf("   Files: %d | Dirs: %d | Total lines: %d | Size: %.1f KB\n",
		data.FileCount, data.DirCount, data.TotalLines, float64(data.TotalSize)/1024)
}

func countLines(path string) int {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return len(strings.Split(string(content), "\n")) - 1
}

func insertIntoTree(root *DirTree, rel string, isDir bool) {
	parts := strings.Split(rel, string(filepath.Separator))
	current := root
	for _, part := range parts {
		found := false
		for _, child := range current.Children {
			if child.Name == part {
				current = child
				found = true
				break
			}
		}
		if !found {
			node := &DirTree{Name: part, IsDir: isDir}
			current.Children = append(current.Children, node)
			current = node
		}
	}
}

func extractAPIRoutes(serverMainPath string) []string {
	content, err := os.ReadFile(serverMainPath)
	if err != nil {
		return nil
	}
	var routes []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, `r.`) {
			continue
		}

		var method string
		switch {
		case strings.HasPrefix(line, "r.Route("):
			method = "ROUTE"
		case strings.HasPrefix(line, "r.Get("):
			method = "GET"
		case strings.HasPrefix(line, "r.Post("):
			method = "POST"
		case strings.HasPrefix(line, "r.Put("):
			method = "PUT"
		case strings.HasPrefix(line, "r.Delete("):
			method = "DELETE"
		case strings.HasPrefix(line, "r.Group("):
			method = "GROUP"
		default:
			continue
		}

		idx := strings.Index(line, `"`)
		if idx >= 0 {
			routes = append(routes, fmt.Sprintf("%-6s %s", method, line[idx:]))
		}
	}
	return routes
}

func extractDomainModels(domainDir string) []DomainModelInfo {
	var models []DomainModelInfo
	entries, err := os.ReadDir(domainDir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		path := filepath.Join(domainDir, e.Name())
		content, _ := os.ReadFile(path)
		text := string(content)
		lines := strings.Split(text, "\n")

		var types []string
		fieldCount := 0
		inStruct := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "type ") && strings.Contains(trimmed, " struct") {
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					types = append(types, parts[1])
				}
				inStruct = true
				continue
			}
			if inStruct {
				if trimmed == "}" {
					inStruct = false
					continue
				}
				if trimmed != "" && !strings.HasPrefix(trimmed, "//") && strings.Contains(trimmed, "`json:") {
					fieldCount++
				}
			}
		}

		if len(types) > 0 {
			models = append(models, DomainModelInfo{
				Package: "domain",
				File:    e.Name(),
				Types:   types,
				Fields:  fieldCount,
			})
		}
	}
	return models
}

func extractGoModules(goModPath string) []string {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return nil
	}
	var modules []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "require") || line == ")" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.Contains(line, "v") && strings.Contains(line, ".") {
			modules = append(modules, line)
		}
	}
	return modules
}

func runCmd(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("(error: %v)", err)
	}
	return string(output)
}

func renderDirTreeHTML(node *DirTree, depth int) string {
	var sb strings.Builder
	indent := strings.Repeat("  ", depth)
	if node.IsDir {
		open := depth == 0 || depth <= 2
		if open {
			sb.WriteString(fmt.Sprintf(`%s<details open>`, indent))
		} else {
			sb.WriteString(fmt.Sprintf(`%s<details>`, indent))
		}
		sb.WriteString(fmt.Sprintf(`<summary><span class="folder-icon">📁</span> %s</summary>`, node.Name))
		for _, child := range node.Children {
			sb.WriteString(renderDirTreeHTML(child, depth+1))
		}
		sb.WriteString(fmt.Sprintf(`%s</details>`, indent))
	} else {
		sb.WriteString(fmt.Sprintf(`%s<div class="file"><span class="file-icon">📄</span> %s</div>`, indent, node.Name))
	}
	return sb.String()
}

const snapshotTemplate = `<!DOCTYPE html>
<html lang="zh-TW">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.ProjectName}} - Snapshot {{.Timestamp}}</title>
<style>
:root {
  --bg: #0d1117;
  --surface: #161b22;
  --surface2: #21262d;
  --border: #30363d;
  --text: #e6edf3;
  --text2: #8b949e;
  --accent: #58a6ff;
  --green: #3fb950;
  --yellow: #d29922;
  --red: #f85149;
  --purple: #bc8cff;
  --orange: #d76f30;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif;
  background: var(--bg);
  color: var(--text);
  line-height: 1.6;
  padding: 20px;
}
.container { max-width: 1200px; margin: 0 auto; }
h1 { font-size: 1.8em; margin-bottom: 4px; }
h2 { font-size: 1.3em; margin: 24px 0 12px; padding-bottom: 8px; border-bottom: 1px solid var(--border); }
h3 { font-size: 1.1em; margin: 16px 0 8px; color: var(--text2); }
.header {
  display: flex; justify-content: space-between; align-items: flex-start;
  padding: 20px; background: var(--surface); border: 1px solid var(--border);
  border-radius: 8px; margin-bottom: 20px; flex-wrap: wrap; gap: 12px;
}
.header .meta { color: var(--text2); font-size: 0.9em; }
.stats-grid {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 12px; margin-bottom: 20px;
}
.stat-card {
  background: var(--surface); border: 1px solid var(--border); border-radius: 8px;
  padding: 16px; text-align: center;
}
.stat-card .value { font-size: 2em; font-weight: 600; color: var(--accent); }
.stat-card .label { font-size: 0.85em; color: var(--text2); margin-top: 4px; }
.card {
  background: var(--surface); border: 1px solid var(--border); border-radius: 8px;
  padding: 20px; margin-bottom: 16px;
}
pre {
  background: var(--surface2); border: 1px solid var(--border); border-radius: 6px;
  padding: 16px; overflow-x: auto; font-size: 0.85em;
  font-family: 'SF Mono', 'Fira Code', monospace; line-height: 1.4;
}
table { width: 100%; border-collapse: collapse; font-size: 0.9em; }
th, td { text-align: left; padding: 8px 12px; border-bottom: 1px solid var(--border); }
th { color: var(--text2); font-weight: 600; }
tr:hover td { background: var(--surface2); }
.badge {
  display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 0.8em;
  font-weight: 500;
}
.badge-go { background: #007d9c33; color: #00add8; }
.badge-js { background: #f0db4f22; color: #f0db4f; }
.badge-css { background: #264de422; color: #264de4; }
.badge-html { background: #e34f2622; color: #e34f26; }
.badge-md { background: #083fa122; color: #519aba; }
.badge-json { background: #29292922; color: #8bc34a; }
.badge-other { background: var(--surface2); color: var(--text2); }
.route { font-family: 'SF Mono', monospace; font-size: 0.9em; }
.model-type { color: var(--purple); }
.model-file { color: var(--text2); font-size: 0.85em; }
.dir-tree details { margin: 4px 0; }
.dir-tree summary { cursor: pointer; padding: 2px 4px; border-radius: 4px; }
.dir-tree summary:hover { background: var(--surface2); }
.dir-tree .file { padding: 2px 4px 2px 24px; color: var(--text2); }
.dir-tree .folder-icon { color: var(--yellow); }
.dir-tree .file-icon { color: var(--text2); }
.tab-bar { display: flex; gap: 4px; margin-bottom: 12px; }
.tab-btn {
  padding: 8px 16px; background: var(--surface2); border: 1px solid var(--border);
  border-radius: 6px 6px 0 0; color: var(--text2); cursor: pointer; font-size: 0.9em;
}
.tab-btn.active { background: var(--surface); color: var(--text); border-bottom-color: var(--surface); }
.tab-content { display: none; }
.tab-content.active { display: block; }
.test-output { max-height: 400px; overflow-y: auto; white-space: pre-wrap; word-break: break-all; }
</style>
</head>
<body>
<div class="container">
  <div class="header">
    <div>
      <h1>📸 {{.ProjectName}}</h1>
      <div class="meta">Generated: {{.Timestamp}}</div>
      <div class="meta">Snapshot via: go run cmd/snapshot/main.go</div>
    </div>
    <div style="text-align:right">
      <div class="meta">📁 {{.FileCount}} files</div>
      <div class="meta">📄 {{.TotalLines}} lines</div>
      <div class="meta">{{printf "%.1f" (divf .TotalSize 1024)}} KB</div>
    </div>
  </div>

  <div class="stats-grid">
    <div class="stat-card"><div class="value">{{.FileCount}}</div><div class="label">Source Files</div></div>
    <div class="stat-card"><div class="value">{{.TotalLines}}</div><div class="label">Total Lines</div></div>
    <div class="stat-card"><div class="value">{{printf "%.1f" (divf .TotalSize 1024)}}</div><div class="label">Size (KB)</div></div>
    <div class="stat-card"><div class="value">{{.DirCount}}</div><div class="label">Directories</div></div>
    <div class="stat-card"><div class="value">{{len .DomainModels}}</div><div class="label">Domain Models</div></div>
    <div class="stat-card"><div class="value">{{len .APIRoutes}}</div><div class="label">API Routes</div></div>
  </div>

  <div class="tab-bar">
    <button class="tab-btn active" onclick="switchTab('overview')">📋 Overview</button>
    <button class="tab-btn" onclick="switchTab('files')">📁 Files</button>
    <button class="tab-btn" onclick="switchTab('api')">🔌 API</button>
    <button class="tab-btn" onclick="switchTab('domain')">🏛️ Domain</button>
    <button class="tab-btn" onclick="switchTab('deps')">📦 Dependencies</button>
    <button class="tab-btn" onclick="switchTab('git')">🔀 Git</button>
    <button class="tab-btn" onclick="switchTab('test')">🧪 Tests</button>
    <button class="tab-btn" onclick="switchTab('tree')">🌳 Tree</button>
  </div>

  <div id="tab-overview" class="tab-content active">
    <div class="card">
      <h2>📋 Project Overview</h2>
      <table>
        <tr><th>Property</th><th>Value</th></tr>
        <tr><td>Project</td><td>{{.ProjectName}}</td></tr>
        <tr><td>Snapshot Time</td><td>{{.Timestamp}}</td></tr>
        <tr><td>Source Files</td><td>{{.FileCount}}</td></tr>
        <tr><td>Directories</td><td>{{.DirCount}}</td></tr>
        <tr><td>Total Lines</td><td>{{.TotalLines}}</td></tr>
        <tr><td>Total Size</td><td>{{printf "%.1f" (divf .TotalSize 1024)}} KB</td></tr>
        <tr><td>Domain Models</td><td>{{len .DomainModels}}</td></tr>
        <tr><td>API Routes</td><td>{{len .APIRoutes}}</td></tr>
        <tr><td>Go Dependencies</td><td>{{len .GoModules}}</td></tr>
        <tr><td>Git Commits (recent)</td><td>{{gitCommitCount .GitLog}}</td></tr>
      </table>
    </div>

    <div class="card">
      <h2>📊 File Type Distribution</h2>
      <table>
        <tr><th>Extension</th><th>Files</th><th>Lines</th><th>%</th></tr>
        {{range $ext, $count := .FilesByExt}}
        <tr>
          <td><span class="badge {{badgeClass $ext}}">{{$ext}}</span></td>
          <td>{{$count}}</td>
          <td>{{index $.LinesByExt $ext}}</td>
          <td>{{printf "%.1f" (percentF (index $.LinesByExt $ext) $.TotalLines)}}%</td>
        </tr>
        {{end}}
      </table>
    </div>
  </div>

  <div id="tab-files" class="tab-content">
    <div class="card">
      <h2>📁 Top Files by Lines</h2>
      <table>
        <tr><th>#</th><th>Path</th><th>Lines</th><th>Size</th><th>Type</th></tr>
        {{range $i, $f := .TopFiles}}
        <tr>
          <td>{{add $i 1}}</td>
          <td><code>{{$f.Path}}</code></td>
          <td>{{$f.Lines}}</td>
          <td>{{printf "%.1f" (divf $f.Size 1024)}} KB</td>
          <td><span class="badge {{badgeClass $f.Extension}}">{{$f.Extension}}</span></td>
        </tr>
        {{end}}
      </table>
    </div>
  </div>

  <div id="tab-api" class="tab-content">
    <div class="card">
      <h2>🔌 API Routes</h2>
      <table>
        <tr><th>Route</th></tr>
        {{range .APIRoutes}}
        <tr><td class="route">{{.}}</td></tr>
        {{end}}
      </table>
    </div>
  </div>

  <div id="tab-domain" class="tab-content">
    <div class="card">
      <h2>🏛️ Domain Models</h2>
      {{range .DomainModels}}
      <div style="margin-bottom: 16px; padding: 12px; background: var(--surface2); border-radius: 6px;">
        <div style="display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:8px;">
          <div>
            <span class="model-file">{{.File}}</span>
            {{range .Types}} <span class="badge badge-go model-type">{{.}}</span>{{end}}
          </div>
          <div style="color:var(--text2);font-size:0.85em;">{{.Fields}} fields</div>
        </div>
      </div>
      {{end}}
    </div>
  </div>

  <div id="tab-deps" class="tab-content">
    <div class="card">
      <h2>📦 Go Dependencies</h2>
      {{if .GoModules}}
      <pre>{{range .GoModules}}{{.}}
{{end}}</pre>
      {{else}}
      <p style="color:var(--text2)">No external dependencies or go.mod not found</p>
      {{end}}
    </div>
  </div>

  <div id="tab-git" class="tab-content">
    <div class="card">
      <h2>🔀 Git Status</h2>
      <h3>Uncommitted (git status --short)</h3>
      <pre>{{if .GitStatus}}{{.GitStatus}}{{else}}No uncommitted changes{{end}}</pre>
      <h3>Recent Commits (git log --oneline -20)</h3>
      <pre>{{if .GitLog}}{{.GitLog}}{{else}}No commits{{end}}</pre>
    </div>
  </div>

  <div id="tab-test" class="tab-content">
    <div class="card">
      <h2>🧪 Test Results</h2>
      <pre class="test-output">{{.TestResult}}</pre>
    </div>
  </div>

  <div id="tab-tree" class="tab-content">
    <div class="card">
      <h2>🌳 Directory Tree</h2>
      <div class="dir-tree">
        {{renderTree .DirTreeJSON}}
      </div>
    </div>
  </div>

  <div style="text-align:center;padding:40px 0;color:var(--text2);font-size:0.9em">
    Generated by go run cmd/snapshot/main.go &mdash; {{.Timestamp}}
  </div>
</div>

<script>
function switchTab(name) {
  document.querySelectorAll('.tab-content').forEach(function(el) { el.classList.remove('active'); });
  document.querySelectorAll('.tab-btn').forEach(function(el) { el.classList.remove('active'); });
  document.getElementById('tab-' + name).classList.add('active');
  var btns = document.querySelectorAll('.tab-btn');
  for (var i = 0; i < btns.length; i++) {
    if (btns[i].getAttribute('onclick').indexOf("'" + name + "'") > -1) {
      btns[i].classList.add('active');
    }
  }
}
</script>
</body>
</html>`
