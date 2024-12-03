package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	ec "github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	bf "github.com/ipfs/boxo/files"
	bp "github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	kr "github.com/ipfs/kubo/client/rpc"
	de "github.com/joho/godotenv"
	ma "github.com/multiformats/go-multiaddr"
)

type Structural struct {
	Cid  string `json:"cid"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type Descriptive struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Technical struct {
	Size   int `json:"size"`
	Width  int `json:"width"`
	Height int `json:"height"`
	Length int `json:"length"`
}

type Attachment struct {
	CID         string `json:"cid"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Extra struct {
	Attachments  []Attachment           `json:"attachments"`
	CustomFields map[string]interface{} `json:"custom_fields"`
}

type Sep struct {
	S Structural  `json:"s"`
	D Descriptive `json:"d"`
	T Technical   `json:"t"`
	X Extra       `json:"x"`
}

func readUnixFile(ctx context.Context, rpc *kr.HttpApi, path string) (bf.File, error) {
	p, err := bp.NewPath(path)
	if err != nil {
		log.Fatal(err)
	}

	n, err := rpc.Unixfs().Get(ctx, p)
	if err != nil {
		return nil, err
	}

	// no dir nav allowed, absolute file only
	// If the file isn't a regular file, nil value will be returned
	file := bf.ToFile(n)
	if file == nil {
		return nil, fmt.Errorf("invalid file")
	}

	return file, nil
}

func getSepFromId(ctx context.Context, kubo *kr.HttpApi, id string) (Sep, error) {

	var sep Sep
	// collect the sep standard to retrieve content
	standard, err := readUnixFile(ctx, kubo, path.Join("/ipfs/", id))
	if err != nil {
		return Sep{}, err
	}

	b, err := io.ReadAll(standard)
	if err != nil {
		return Sep{}, err
	}

	err = json.Unmarshal(b, &sep)
	if err != nil {
		return Sep{}, err
	}

	return sep, nil

}

// TODO refactor all these to modules

func fetchHandler(kubo *kr.HttpApi, eth *ec.Client) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		sub := chi.URLParam(r, "sub")
		ctx := context.Background()
		name := sub

		c, err := cid.Decode(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// if the cid is a sep-001 standard switch the file served
		if sep, err := getSepFromId(ctx, kubo, c.String()); err == nil {
			log.Printf("Matched sep with id %s", id)
			id = sep.S.Cid

			// if valid sep standard
			if sub == "" {
				sub = sep.S.Path
				subSplit := strings.Split(sub, "/")
				name = subSplit[len(subSplit)-1]
				log.Printf("Using default standard index %s", sub)
			}
		}

		log.Printf("Service file %s", sub)
		file, err := readUnixFile(ctx, kubo, path.Join("/ipfs/", id, sub))
		// // no dir nav allowed, absolute file only
		// // If the file isn't a regular file, nil value will be returned
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// ServeContent replies to the request using the content in the provided ReadSeeker.
		// The main benefit of ServeContent over io.Copy is that it handles Range requests properly, sets the MIME type,
		// and handles If-Match, If-Unmodified-Since, If-None-Match, If-Modified-Since, and If-Range requests.
		http.ServeContent(w, r, name, file.ModTime(), file)
	}
}

func metaHandler(kubo *kr.HttpApi) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		ctx := context.Background()

		type Partial struct {
			Type       string
			Attachment []Attachment
			Meta       Descriptive
		}

		sep, err := getSepFromId(ctx, kubo, id)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// serve json partial metadata
		render.JSON(w, r, Partial{sep.S.Type, sep.X.Attachments, sep.D})
	}
}

// handler
func main() {

	r := chi.NewRouter()
	err := de.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file: %s", err)
	}

	r.Use(middleware.Logger)
	// r.Use(middleware.NoCache)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(time.Second * 60))
	r.Use(middleware.Compress(6, "/*"))
	r.Use(cors.Handler(cors.Options{
		// AllowedOrigins:   []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins: []string{"https://*", "http://*"},
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		// ExposedHeaders:   []string{},
		AllowCredentials: false,
		// preflight cache maximum value not ignored by any of major browsers
		MaxAge: 300,
	}))

	localNodeAddress := os.Getenv("IPFS_API")
	local, err := ma.NewMultiaddr(localNodeAddress)
	if err != nil {
		log.Fatal(err)
	}

	kubo, err := kr.NewApi(local)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Connected to %s", localNodeAddress)
	client, err := ec.Dial(os.Getenv("RPC_ENDPOINT"))
	if err != nil {
		log.Fatal(err)
	}

	r.Get("/fetch/{id}/", fetchHandler(kubo, client))
	r.Get("/fetch/{id}/{sub}", fetchHandler(kubo, client))
	r.Get("/metadata/{id}/", metaHandler(kubo))

	// Start the node on port 8080, and log any errors
	port := fmt.Sprintf(":%s", os.Getenv("NODE_PORT"))
	log.Printf("Running node on port %s", port)
	log.Panic(http.ListenAndServe(port, r))
}
