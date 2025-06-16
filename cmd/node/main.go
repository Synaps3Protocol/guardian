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
	"strconv"
	"strings"
	"time"

	ec "github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	lru "github.com/hashicorp/golang-lru/v2"
	bf "github.com/ipfs/boxo/files"
	bp "github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	kr "github.com/ipfs/kubo/client/rpc"
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
	Cid         string `json:"cid"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Extra struct {
	Attachments  []Attachment           `json:"attachments"`
	CustomFields map[string]interface{} `json:"custom_fields"`
}

// SEP-002
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

func getSepFromId(ctx context.Context, kubo *kr.HttpApi, cache *lru.Cache[string, Sep], id string) (Sep, error) {

	var sep Sep
	// check if id is in cache
	if v, ok := cache.Get(id); ok {
		log.Printf("Using cache for id: %s", id)
		return v, nil
	}

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

	cache.Add(id, sep)
	return sep, nil

}

// TODO refactor all these to modules

func contentHandler(kubo *kr.HttpApi, eth *ec.Client, cache *lru.Cache[string, Sep]) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		sub := chi.URLParam(r, "sub")
		name := sub

		timeout := time.Second * 10
		parent := context.Background() // if do not find the data before 5 seconds, fail..
		ctx, cancel := context.WithTimeout(parent, timeout)
		defer cancel()

		c, err := cid.Decode(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		log.Printf("Attempt to find id %s", id)
		// if the cid is a sep-001 standard switch the file served
		if sep, err := getSepFromId(ctx, kubo, cache, c.String()); err == nil {
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

		log.Printf("Service file %s from %s", sub, id)
		file, err := readUnixFile(ctx, kubo, path.Join("/ipfs/", id, sub))
		// no dir nav allowed, absolute file only
		// If the file isn't a regular file, nil value will be returned
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

func metaHandler(kubo *kr.HttpApi, cache *lru.Cache[string, Sep]) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		// TODO refactor all this
		parent := context.Background()
		timeout := time.Second * 10
		ctx, cancel := context.WithTimeout(parent, timeout)
		defer cancel()

		// Embedding descriptive and extra
		type Data struct {
			Descriptive
			Technical
			Extra
		}

		type Partial struct {
			Type string
			Data Data
		}

		log.Printf("Attempt to find id %s", id)
		sep, err := getSepFromId(ctx, kubo, cache, id)
		if err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// serve json partial metadata
		data := Data{sep.D, sep.T, sep.X}
		render.JSON(w, r, Partial{sep.S.Type, data})
	}
}

func health() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

// handler
func main() {

	r := chi.NewRouter()
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

	cacheSize, err := strconv.Atoi(os.Getenv("LRU_CACHE_SIZE"))
	if err != nil {
		log.Fatal(err)
	}

	lruCache, err := lru.New[string, Sep](cacheSize)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Cache initialized with size to %d", cacheSize)
	r.Get("/content/{id}/", contentHandler(kubo, client, lruCache))
	r.Get("/content/{id}/{sub}", contentHandler(kubo, client, lruCache))
	r.Get("/metadata/{id}/", metaHandler(kubo, lruCache))
	r.Get("/healthcheck/", health())

	// Start the node on port 8080, and log any errors
	port := fmt.Sprintf(":%s", os.Getenv("NODE_PORT"))
	log.Printf("Running node on port %s", port)
	log.Panic(http.ListenAndServe(port, r))
}
