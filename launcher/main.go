// Quintet — lanceur Windows : sert le jeu en local et l'ouvre dans une fenêtre d'application.
package main

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"
)

//go:embed web
var webFS embed.FS

// Port fixe : l'adresse ne change jamais, donc les sauvegardes du navigateur sont conservées.
const addr = "127.0.0.1:47815"
const url = "http://" + addr + "/"

var lastPing atomic.Int64

// Adresses de la version en ligne : le jeu est mis à jour à chaque lancement s'il y a une connexion.
var onlineURLs = []string{"https://yatoohpro.github.io/Qintet/index.html", "https://yatoohpro.github.io/qintet/index.html"}

func cacheFile() string { return filepath.Join(filepath.Dir(profileDir()), "index-en-ligne.html") }

// Télécharge la dernière version en ligne (12 s max) et la garde en cache pour les lancements hors ligne.
func refresh() {
	cl := &http.Client{Timeout: 12 * time.Second}
	for _, u := range onlineURLs {
		r, err := cl.Get(u + "?t=" + time.Now().Format("20060102150405"))
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
		r.Body.Close()
		if err == nil && r.StatusCode == 200 && bytes.Contains(b, []byte("QUINTET")) && bytes.Contains(b, []byte("function render(")) {
			os.WriteFile(cacheFile(), b, 0o644)
			return
		}
	}
}

// Page du jeu : version en ligne la plus récente si disponible, sinon la copie intégrée ; mode bureau activé.
func gamePage() []byte {
	b, err := os.ReadFile(cacheFile())
	if err != nil || len(b) < 1000 {
		b, _ = webFS.ReadFile("web/index.html")
	}
	if !bytes.Contains(b, []byte("window.__desktop=1")) {
		b = bytes.Replace(b, []byte("<meta charset=\"utf-8\">"), []byte("<meta charset=\"utf-8\">\n<script>window.__desktop=1</script>"), 1)
	}
	return b
}

func main() {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// Quintet tourne déjà : on ouvre simplement une nouvelle fenêtre.
		openWindow()
		return
	}
	sub, _ := fs.Sub(webFS, "web")
	files := http.FileServer(http.FS(sub))
	mux := http.NewServeMux()
	mux.HandleFunc("/__refresh", func(w http.ResponseWriter, r *http.Request) {
		refresh()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/__ping", func(w http.ResponseWriter, r *http.Request) {
		lastPing.Store(time.Now().Unix())
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(gamePage())
			return
		}
		files.ServeHTTP(w, r)
	}))
	refresh()
	go http.Serve(ln, mux)

	lastPing.Store(time.Now().Unix())
	openWindow()

	// Le serveur s'arrête quand aucune fenêtre du jeu n'a donné signe de vie depuis 25 secondes.
	grace := time.Now().Add(60 * time.Second)
	for {
		time.Sleep(5 * time.Second)
		if time.Now().Before(grace) {
			continue
		}
		if time.Now().Unix()-lastPing.Load() > 25 {
			return
		}
	}
}

func profileDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	d := filepath.Join(base, "Quintet", "profil")
	os.MkdirAll(d, 0o755)
	return d
}

func openWindow() {
	args := []string{"--app=" + url, "--user-data-dir=" + profileDir(), "--no-first-run", "--no-default-browser-check", "--window-size=1400,900"}
	if runtime.GOOS == "windows" {
		pf := []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LOCALAPPDATA")}
		cands := []string{}
		for _, p := range pf {
			if p == "" {
				continue
			}
			cands = append(cands,
				filepath.Join(p, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(p, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(p, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"))
		}
		for _, c := range cands {
			if _, err := os.Stat(c); err == nil {
				if exec.Command(c, args...).Start() == nil {
					return
				}
			}
		}
		// Dernier recours : navigateur par défaut.
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		return
	}
	for _, c := range []string{"microsoft-edge", "google-chrome", "chromium", "chromium-browser"} {
		if p, err := exec.LookPath(c); err == nil {
			if exec.Command(p, args...).Start() == nil {
				return
			}
		}
	}
	exec.Command("xdg-open", url).Start()
}
