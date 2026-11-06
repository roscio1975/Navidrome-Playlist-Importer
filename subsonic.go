package main

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Song is a track in the Navidrome library.
type Song struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Duration int    `json:"duration,omitempty"`

	// internal fields computed in memory (not serialized)
	T   string          `json:"-"`
	A   string          `json:"-"`
	AL  string          `json:"-"`
	TV  []string        `json:"-"`
	TG  map[string]bool `json:"-"`
	TOK []string        `json:"-"`
}

type PlaylistRef struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SongCount int    `json:"songCount"`
}

type artistRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type albumRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type apiResponse struct {
	Status  string    `json:"status"`
	Version string    `json:"version"`
	Error   *apiError `json:"error"`

	Artists *struct {
		Index []struct {
			Artist []artistRef `json:"artist"`
		} `json:"index"`
	} `json:"artists"`
	Artist *struct {
		Album []albumRef `json:"album"`
	} `json:"artist"`
	Album *struct {
		Song []Song `json:"song"`
	} `json:"album"`
	Playlists *struct {
		Playlist []PlaylistRef `json:"playlist"`
	} `json:"playlists"`
	Playlist      *PlaylistRef `json:"playlist"`
	Song          *Song        `json:"song"`
	SearchResult3 *struct {
		Song []Song `json:"song"`
	} `json:"searchResult3"`
}

// Client is a minimal client for the Navidrome Subsonic API.
type Client struct {
	Base string
	User string
	Pass string
	HTTP *http.Client
}

func NewClient(rawURL, user, pass string) (*Client, error) {
	base := strings.TrimSpace(rawURL)
	if base == "" {
		return nil, errors.New("server URL is empty")
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	base = strings.TrimRight(base, "/")
	return &Client{
		Base: base,
		User: user,
		Pass: pass,
		HTTP: &http.Client{Timeout: 90 * time.Second},
	}, nil
}

func randomSalt() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func md5hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func (c *Client) call(method string, params url.Values) (*apiResponse, error) {
	salt := randomSalt()
	q := url.Values{}
	for k, vs := range params {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	q.Set("u", c.User)
	q.Set("t", md5hex(c.Pass+salt))
	q.Set("s", salt)
	q.Set("v", "1.16.1")
	q.Set("c", "navidrome-import")
	q.Set("f", "json")

	endpoint := c.Base + "/rest/" + method + ".view"

	var (
		resp *http.Response
		err  error
	)
	// Use POST when the request is long (playlists with many tracks).
	if len(q.Encode()) > 1800 {
		resp, err = c.HTTP.PostForm(endpoint, q)
	} else {
		resp, err = c.HTTP.Get(endpoint + "?" + q.Encode())
	}
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", c.Base, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 && len(body) == 0 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, method)
	}

	var wrapper struct {
		Response *apiResponse `json:"subsonic-response"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("invalid response from %s: %w", method, err)
	}
	if wrapper.Response == nil {
		return nil, fmt.Errorf("empty response from %s", method)
	}
	if wrapper.Response.Status != "ok" {
		if wrapper.Response.Error != nil {
			return nil, fmt.Errorf("%s: %s (code %d)", method, wrapper.Response.Error.Message, wrapper.Response.Error.Code)
		}
		return nil, fmt.Errorf("%s: response %q", method, wrapper.Response.Status)
	}
	return wrapper.Response, nil
}

func (c *Client) Ping() error {
	_, err := c.call("ping", nil)
	return err
}

func (c *Client) GetArtists() ([]artistRef, error) {
	r, err := c.call("getArtists", nil)
	if err != nil {
		return nil, err
	}
	if r.Artists == nil {
		return nil, nil
	}
	var out []artistRef
	for _, idx := range r.Artists.Index {
		out = append(out, idx.Artist...)
	}
	return out, nil
}

func (c *Client) GetArtist(id string) ([]albumRef, error) {
	r, err := c.call("getArtist", url.Values{"id": {id}})
	if err != nil {
		return nil, err
	}
	if r.Artist == nil {
		return nil, nil
	}
	return r.Artist.Album, nil
}

func (c *Client) GetAlbum(id string) ([]Song, error) {
	r, err := c.call("getAlbum", url.Values{"id": {id}})
	if err != nil {
		return nil, err
	}
	if r.Album == nil {
		return nil, nil
	}
	return r.Album.Song, nil
}

func (c *Client) GetPlaylists() ([]PlaylistRef, error) {
	r, err := c.call("getPlaylists", nil)
	if err != nil {
		return nil, err
	}
	if r.Playlists == nil {
		return nil, nil
	}
	return r.Playlists.Playlist, nil
}

func (c *Client) GetPlaylist(id string) (*PlaylistRef, error) {
	r, err := c.call("getPlaylist", url.Values{"id": {id}})
	if err != nil {
		return nil, err
	}
	return r.Playlist, nil
}

func (c *Client) GetSong(id string) (*Song, error) {
	r, err := c.call("getSong", url.Values{"id": {id}})
	if err != nil {
		return nil, err
	}
	if r.Song == nil {
		return nil, fmt.Errorf("track %q not found", id)
	}
	return r.Song, nil
}

func (c *Client) CreatePlaylist(name string, songIDs []string) (*PlaylistRef, error) {
	p := url.Values{"name": {name}}
	for _, id := range songIDs {
		p.Add("songId", id)
	}
	r, err := c.call("createPlaylist", p)
	if err != nil {
		return nil, err
	}
	return r.Playlist, nil
}

func (c *Client) UpdatePlaylist(id, name string, songIDs []string) (*PlaylistRef, error) {
	p := url.Values{"playlistId": {id}}
	if name != "" {
		p.Set("name", name)
	}
	for _, sid := range songIDs {
		p.Add("songId", sid)
	}
	r, err := c.call("createPlaylist", p)
	if err != nil {
		return nil, err
	}
	return r.Playlist, nil
}

func (c *Client) DeletePlaylist(id string) error {
	_, err := c.call("deletePlaylist", url.Values{"id": {id}})
	return err
}

func (c *Client) Search(query string) ([]Song, error) {
	r, err := c.call("search3", url.Values{
		"query":       {query},
		"songCount":   {"20"},
		"artistCount": {"0"},
		"albumCount":  {"0"},
		"songOffset":  {"0"},
	})
	if err != nil {
		return nil, err
	}
	var out []Song
	if r.SearchResult3 != nil {
		out = r.SearchResult3.Song
	}
	return out, nil
}
