package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
	"google.golang.org/api/youtube/v3"
)

/*
All youtube format qualities:
highres
hd1080
hd720
large
medium
small
min
max
*/

type playerConfig []byte
type FormatList []Format
type Thumbnails []Thumbnail
type ErrUnexpectedStatusCode int
type DecipherOperation func([]byte) []byte

type playerCache struct {
	key       string
	expiredAt time.Time
	config    playerConfig
}

type inntertubeContext struct {
	Client innertubeClient `json:"client"`
}

type innertubeClient struct {
	HL            string `json:"hl"`
	GL            string `json:"gl"`
	ClientName    string `json:"clientName"`
	ClientVersion string `json:"clientVersion"`
}

type innertubeRequest struct {
	VideoID         string            `json:"videoId,omitempty"`
	BrowseID        string            `json:"browseId,omitempty"`
	Continuation    string            `json:"continuation,omitempty"`
	Context         inntertubeContext `json:"context"`
	PlaybackContext playbackContext   `json:"playbackContext,omitempty"`
}

type playbackContext struct {
	ContentPlaybackContext contentPlaybackContext `json:"contentPlaybackContext"`
}

type contentPlaybackContext struct {
	SignatureTimestamp string `json:"signatureTimestamp"`
}

// Client offers methods to download video metadata and video streams.
type Client struct {
	// Debug enables debugging output through log package
	Debug bool

	// HTTPClient can be used to set a custom HTTP client.
	// If not set, http.DefaultClient will be used
	HTTPClient *http.Client

	// playerCache caches the JavaScript code of a player response
	playerCache playerCache
}

type Video struct {
	ID              string
	Title           string
	Description     string
	Author          string
	Duration        time.Duration
	PublishDate     time.Time
	Formats         FormatList
	Thumbnails      Thumbnails
	DASHManifestURL string // URI of the DASH manifest file
	HLSManifestURL  string // URI of the HLS manifest file
}

type Format struct {
	ItagNo           int    `json:"itag"`
	URL              string `json:"url"`
	MimeType         string `json:"mimeType"`
	Quality          string `json:"quality"`
	Cipher           string `json:"signatureCipher"`
	Bitrate          int    `json:"bitrate"`
	FPS              int    `json:"fps"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	LastModified     string `json:"lastModified"`
	ContentLength    int64  `json:"contentLength,string"`
	QualityLabel     string `json:"qualityLabel"`
	ProjectionType   string `json:"projectionType"`
	AverageBitrate   int    `json:"averageBitrate"`
	AudioQuality     string `json:"audioQuality"`
	ApproxDurationMs string `json:"approxDurationMs"`
	AudioSampleRate  string `json:"audioSampleRate"`
	AudioChannels    int    `json:"audioChannels"`

	// InitRange is only available for adaptive formats
	InitRange *struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"initRange"`

	// IndexRange is only available for adaptive formats
	IndexRange *struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"indexRange"`
}

type playerResponseData struct {
	PlayabilityStatus struct {
		Status          string `json:"status"`
		Reason          string `json:"reason"`
		PlayableInEmbed bool   `json:"playableInEmbed"`
		Miniplayer      struct {
			MiniplayerRenderer struct {
				PlaybackMode string `json:"playbackMode"`
			} `json:"miniplayerRenderer"`
		} `json:"miniplayer"`
		ContextParams string `json:"contextParams"`
	} `json:"playabilityStatus"`
	StreamingData struct {
		ExpiresInSeconds string   `json:"expiresInSeconds"`
		Formats          []Format `json:"formats"`
		AdaptiveFormats  []Format `json:"adaptiveFormats"`
		DashManifestURL  string   `json:"dashManifestUrl"`
		HlsManifestURL   string   `json:"hlsManifestUrl"`
	} `json:"streamingData"`
	VideoDetails struct {
		VideoID          string   `json:"videoId"`
		Title            string   `json:"title"`
		LengthSeconds    string   `json:"lengthSeconds"`
		Keywords         []string `json:"keywords"`
		ChannelID        string   `json:"channelId"`
		IsOwnerViewing   bool     `json:"isOwnerViewing"`
		ShortDescription string   `json:"shortDescription"`
		IsCrawlable      bool     `json:"isCrawlable"`
		Thumbnail        struct {
			Thumbnails []Thumbnail `json:"thumbnails"`
		} `json:"thumbnail"`
		AverageRating     float64 `json:"averageRating"`
		AllowRatings      bool    `json:"allowRatings"`
		ViewCount         string  `json:"viewCount"`
		Author            string  `json:"author"`
		IsPrivate         bool    `json:"isPrivate"`
		IsUnpluggedCorpus bool    `json:"isUnpluggedCorpus"`
		IsLiveContent     bool    `json:"isLiveContent"`
	} `json:"videoDetails"`
	Microformat struct {
		PlayerMicroformatRenderer struct {
			Thumbnail struct {
				Thumbnails []struct {
					URL    string `json:"url"`
					Width  int    `json:"width"`
					Height int    `json:"height"`
				} `json:"thumbnails"`
			} `json:"thumbnail"`
			Title struct {
				SimpleText string `json:"simpleText"`
			} `json:"title"`
			Description struct {
				SimpleText string `json:"simpleText"`
			} `json:"description"`
			LengthSeconds      string   `json:"lengthSeconds"`
			OwnerProfileURL    string   `json:"ownerProfileUrl"`
			ExternalChannelID  string   `json:"externalChannelId"`
			IsFamilySafe       bool     `json:"isFamilySafe"`
			AvailableCountries []string `json:"availableCountries"`
			IsUnlisted         bool     `json:"isUnlisted"`
			HasYpcMetadata     bool     `json:"hasYpcMetadata"`
			ViewCount          string   `json:"viewCount"`
			Category           string   `json:"category"`
			PublishDate        string   `json:"publishDate"`
			OwnerChannelName   string   `json:"ownerChannelName"`
			UploadDate         string   `json:"uploadDate"`
		} `json:"playerMicroformatRenderer"`
	} `json:"microformat"`
}

type Thumbnail struct {
	URL    string
	Width  uint
	Height uint
}

const defaultCacheExpiration = time.Minute * time.Duration(5)

type ClientType string

const (
	Web            ClientType = "WEB"
	EmbeddedClient ClientType = "WEB_EMBEDDED_PLAYER"
)

const dateFormat = "2006-01-02"

const (
	jsvarStr   = "[a-zA-Z_\\$][a-zA-Z_0-9]*"
	reverseStr = ":function\\(a\\)\\{" +
		"(?:return )?a\\.reverse\\(\\)" +
		"\\}"
	spliceStr = ":function\\(a,b\\)\\{" +
		"a\\.splice\\(0,b\\)" +
		"\\}"
	swapStr = ":function\\(a,b\\)\\{" +
		"var c=a\\[0\\];a\\[0\\]=a\\[b(?:%a\\.length)?\\];a\\[b(?:%a\\.length)?\\]=c(?:;return a)?" +
		"\\}"
)

const (
	ErrCipherNotFound             = constError("cipher not found")
	ErrUrlNotFound                = constError("URL not found")
	ErrSignatureTimestampNotFound = constError("signature timestamp not found")
	ErrInvalidCharactersInVideoID = constError("invalid characters in video id")
	ErrVideoIDMinLength           = constError("the video id must be at least 10 characters long")
	ErrReadOnClosedResBody        = constError("http: read on closed response body")
	ErrNotPlayableInEmbed         = constError("embedding of this video has been disabled")
	ErrLoginRequired              = constError("login required to confirm your age")
	ErrVideoPrivate               = constError("user restricted access to this video")
	ErrInvalidPlaylist            = constError("no playlist detected or invalid playlist ID")
)

type constError string

func (e constError) Error() string {
	return string(e)
}

type ErrPlayabiltyStatus struct {
	Status string
	Reason string
}

func (err ErrPlayabiltyStatus) Error() string {
	return fmt.Sprintf("cannot playback and download, status: %s, reason: %s", err.Status, err.Reason)
}

type ErrPlaylistStatus struct {
	Reason string
}

func (err ErrPlaylistStatus) Error() string {
	return fmt.Sprintf("could not load playlist: %s", err.Reason)
}

var basejsPattern = regexp.MustCompile(`(/s/player/\w+/player_ias.vflset/\w+/base.js)`)

// we may use \d{5} instead of \d+ since currently its 5 digits, but i can't be sure it will be 5 digits always
var signatureRegexp = regexp.MustCompile(`(?m)(?:^|,)(?:signatureTimestamp:)(\d+)`)

var (
	nFunctionNameRegexp = regexp.MustCompile("\\.get\\(\"n\"\\)\\)&&\\(b=([a-zA-Z0-9]{3})\\[(\\d+)\\](.+)\\|\\|([a-zA-Z0-9]{3})")
	actionsObjRegexp    = regexp.MustCompile(fmt.Sprintf("var (%s)=\\{((?:(?:%s%s|%s%s|%s%s),?\\n?)+)\\};", jsvarStr, jsvarStr, swapStr, jsvarStr, spliceStr, jsvarStr, reverseStr))

	actionsFuncRegexp = regexp.MustCompile(fmt.Sprintf(
		"function(?: %s)?\\(a\\)\\{"+
			"a=a\\.split\\(\"\"\\);\\s*"+
			"((?:(?:a=)?%s\\.%s\\(a,\\d+\\);)+)"+
			"return a\\.join\\(\"\"\\)"+
			"\\}", jsvarStr, jsvarStr, jsvarStr))

	reverseRegexp = regexp.MustCompile(fmt.Sprintf("(?m)(?:^|,)(%s)%s", jsvarStr, reverseStr))
	spliceRegexp  = regexp.MustCompile(fmt.Sprintf("(?m)(?:^|,)(%s)%s", jsvarStr, spliceStr))
	swapRegexp    = regexp.MustCompile(fmt.Sprintf("(?m)(?:^|,)(%s)%s", jsvarStr, swapStr))
)

var videoRegexpList = []*regexp.Regexp{
	regexp.MustCompile(`(?:v|embed|shorts|watch\?v)(?:=|/)([^"&?/=%]{11})`),
	regexp.MustCompile(`(?:=|/)([^"&?/=%]{11})`),
	regexp.MustCompile(`([^"&?/=%]{11})`),
}

func (err ErrUnexpectedStatusCode) Error() string {
	return fmt.Sprintf("unexpected status code: %d", err)
}

// Retrieve playlistItems in the specified playlist
func playlistItemsList(service *youtube.Service, part []string, playlistId string, pageToken string) *youtube.PlaylistItemListResponse {
	call := service.PlaylistItems.List(part)
	call = call.PlaylistId(playlistId)
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}
	response, err := call.Do()
	log.Println(err)
	return response
}

// GetVideo fetches video metadata
func (c *Client) GetVideo(url string) (*Video, error) {
	return c.GetVideoContext(context.Background(), url)
}

// GetVideoContext fetches video metadata with a context
func (c *Client) GetVideoContext(ctx context.Context, url string) (*Video, error) {
	id, err := ExtractVideoID(url)
	if err != nil {
		return nil, fmt.Errorf("extractVideoID failed: %w", err)
	}
	return c.videoFromID(ctx, id)
}

func (c *Client) videoFromID(ctx context.Context, id string) (*Video, error) {
	body, err := c.videoDataByInnertube(ctx, id, Web)
	if err != nil {
		return nil, err
	}

	v := &Video{
		ID: id,
	}

	err = v.parseVideoInfo(body)
	// return early if all good
	if err == nil {
		return v, nil
	}

	// If the uploader has disabled embedding the video on other sites, parse video page
	if err == ErrNotPlayableInEmbed {
		// additional parameters are required to access clips with sensitiv content
		html, err := c.httpGetBodyBytes(ctx, "https://www.youtube.com/watch?v="+id+"&bpctr=9999999999&has_verified=1")
		if err != nil {
			return nil, err
		}

		return v, v.parseVideoPage(html)
	}

	// If the uploader marked the video as inappropriate for some ages, use embed player
	if err == ErrLoginRequired {
		bodyEmbed, errEmbed := c.videoDataByInnertube(ctx, id, EmbeddedClient)
		if errEmbed == nil {
			errEmbed = v.parseVideoInfo(bodyEmbed)
		}

		if errEmbed == nil {
			return v, nil
		}

		// private video clearly not age-restricted and thus should be explicit
		if errEmbed == ErrVideoPrivate {
			return v, errEmbed
		}

		// wrapping error so its clear whats happened
		return v, fmt.Errorf("can't bypass age restriction: %w", errEmbed)
	}

	// undefined error
	return v, err
}

var playerResponsePattern = regexp.MustCompile(`var ytInitialPlayerResponse\s*=\s*(\{.+?\});`)

func (v *Video) parseVideoPage(body []byte) error {
	initialPlayerResponse := playerResponsePattern.FindSubmatch(body)
	if initialPlayerResponse == nil || len(initialPlayerResponse) < 2 {
		return errors.New("no ytInitialPlayerResponse found in the server's answer")
	}

	var prData playerResponseData
	if err := json.Unmarshal(initialPlayerResponse[1], &prData); err != nil {
		return fmt.Errorf("unable to parse player response JSON: %w", err)
	}

	if err := v.isVideoFromPageDownloadable(prData); err != nil {
		return err
	}

	return v.extractDataFromPlayerResponse(prData)
}

func (v *Video) isVideoFromPageDownloadable(prData playerResponseData) error {
	return v.isVideoDownloadable(prData, true)
}

func (v *Video) parseVideoInfo(body []byte) error {
	var prData playerResponseData
	if err := json.Unmarshal(body, &prData); err != nil {
		return fmt.Errorf("unable to parse player response JSON: %w", err)
	}

	if err := v.isVideoFromInfoDownloadable(prData); err != nil {
		return err
	}

	return v.extractDataFromPlayerResponse(prData)
}

var innertubeClientInfo = map[ClientType]map[string]string{
	// might add ANDROID and other in future, but i don't see reason yet
	Web: {
		"version": "2.20210617.01.00",
		"key":     "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8",
	},
	EmbeddedClient: {
		"version": "1.19700101",
		// seems like same key works for both clients
		"key": "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8",
	},
}

// FindByQuality returns the first format matching Quality or QualityLabel
func (list FormatList) FindByQuality(quality string) *Format {
	for i := range list {
		if list[i].Quality == quality || list[i].QualityLabel == quality {
			return &list[i]
		}
	}
	return nil
}

func (v *Video) extractDataFromPlayerResponse(prData playerResponseData) error {
	v.Title = prData.VideoDetails.Title
	v.Description = prData.VideoDetails.ShortDescription
	v.Author = prData.VideoDetails.Author
	v.Thumbnails = prData.VideoDetails.Thumbnail.Thumbnails

	if seconds, _ := strconv.Atoi(prData.Microformat.PlayerMicroformatRenderer.LengthSeconds); seconds > 0 {
		v.Duration = time.Duration(seconds) * time.Second
	}

	if str := prData.Microformat.PlayerMicroformatRenderer.PublishDate; str != "" {
		v.PublishDate, _ = time.Parse(dateFormat, str)
	}

	// Assign Streams
	v.Formats = append(prData.StreamingData.Formats, prData.StreamingData.AdaptiveFormats...)
	if len(v.Formats) == 0 {
		return errors.New("no formats found in the server's answer")
	}

	// Sort formats by bitrate
	sort.SliceStable(v.Formats, v.SortBitrateDesc)

	v.HLSManifestURL = prData.StreamingData.HlsManifestURL
	v.DASHManifestURL = prData.StreamingData.DashManifestURL

	return nil
}

func (v *Video) SortBitrateDesc(i int, j int) bool {
	return v.Formats[i].Bitrate > v.Formats[j].Bitrate
}

func (v *Video) SortBitrateAsc(i int, j int) bool {
	return v.Formats[i].Bitrate < v.Formats[j].Bitrate
}

func (v *Video) isVideoFromInfoDownloadable(prData playerResponseData) error {
	return v.isVideoDownloadable(prData, false)
}

func (v *Video) isVideoDownloadable(prData playerResponseData, isVideoPage bool) error {
	// Check if video is downloadable
	switch prData.PlayabilityStatus.Status {
	case "OK":
		return nil
	case "LOGIN_REQUIRED":
		// for some reason they use same status message for age-restricted and private videos
		if strings.HasPrefix(prData.PlayabilityStatus.Reason, "This video is private") {
			return ErrVideoPrivate
		}
		return ErrLoginRequired
	}

	if !isVideoPage && !prData.PlayabilityStatus.PlayableInEmbed {
		return ErrNotPlayableInEmbed
	}

	return &ErrPlayabiltyStatus{
		Status: prData.PlayabilityStatus.Status,
		Reason: prData.PlayabilityStatus.Reason,
	}
}

func (c *Client) videoDataByInnertube(ctx context.Context, id string, clientType ClientType) ([]byte, error) {
	config, err := c.getPlayerConfig(ctx, id)
	if err != nil {
		return nil, err
	}

	// fetch sts first
	sts, err := config.getSignatureTimestamp()
	if err != nil {
		return nil, err
	}

	data, keyToken := prepareInnertubeVideoData(id, sts, clientType)
	return c.httpPostBodyBytes(ctx, "https://www.youtube.com/youtubei/v1/player?key="+keyToken, data)
}

func prepareInnertubeVideoData(videoID string, sts string, clientType ClientType) (innertubeRequest, string) {
	context, key := prepareInnertubeContext(clientType)

	return innertubeRequest{
		VideoID: videoID,
		Context: context,
		PlaybackContext: playbackContext{
			ContentPlaybackContext: contentPlaybackContext{
				SignatureTimestamp: sts,
			},
		},
	}, key
}

func prepareInnertubeContext(clientType ClientType) (inntertubeContext, string) {
	cInfo, ok := innertubeClientInfo[clientType]
	if !ok {
		// if provided clientType not exist - use Web as fallback option
		clientType = Web
		cInfo = innertubeClientInfo[clientType]
	}

	return inntertubeContext{
		Client: innertubeClient{
			HL:            "en",
			GL:            "US",
			ClientName:    string(clientType),
			ClientVersion: cInfo["version"],
		},
	}, cInfo["key"]
}

// httpPostBodyBytes reads the whole HTTP body and returns it
func (c *Client) httpPostBodyBytes(ctx context.Context, url string, body interface{}) ([]byte, error) {
	resp, err := c.httpPost(ctx, url, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// httpPost does a HTTP POST request with a body, checks the response to be a 200 OK and returns it
func (c *Client) httpPost(ctx context.Context, url string, body interface{}) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	resp, err := c.httpDo(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, ErrUnexpectedStatusCode(resp.StatusCode)
	}
	return resp, nil
}

func (config playerConfig) getSignatureTimestamp() (string, error) {
	result := signatureRegexp.FindSubmatch(config)
	if result == nil {
		return "", ErrSignatureTimestampNotFound
	}

	return string(result[1]), nil
}

// ExtractVideoID extracts the videoID from the given string
func ExtractVideoID(videoID string) (string, error) {
	if strings.Contains(videoID, "youtu") || strings.ContainsAny(videoID, "\"?&/<%=") {
		for _, re := range videoRegexpList {
			if isMatch := re.MatchString(videoID); isMatch {
				subs := re.FindStringSubmatch(videoID)
				videoID = subs[1]
			}
		}
	}

	if strings.ContainsAny(videoID, "?&/<%=") {
		return "", ErrInvalidCharactersInVideoID
	}
	if len(videoID) < 10 {
		return "", ErrVideoIDMinLength
	}

	return videoID, nil
}

// GetStreamURL returns the url for a specific format
func (c *Client) GetStreamURL(video *Video, format *Format) (string, error) {
	return c.GetStreamURLContext(context.Background(), video, format)
}

// GetStreamURLContext returns the url for a specific format with a context
func (c *Client) GetStreamURLContext(ctx context.Context, video *Video, format *Format) (string, error) {
	log.Println("FORMAT URL: " + format.URL)
	if format.URL != "" {
		//return format.URL, nil
	}

	// cipher := format.Cipher
	// if cipher == "" {
	// 	return "", ErrCipherNotFound
	// }
	cipher := format.Cipher

	var qualities = []string{"highres", "hd1080", "hd720", "large", "medium", "small", "min", "max"}
	if cipher == "" {
		for _, element := range qualities {
			format = video.Formats.FindByQuality(element)
			if cipher != "" {
				break
			}
		}
	}

	uri, err := c.decipherURL(ctx, video.ID, cipher)
	if err != nil {
		return "", err
	}

	log.Println("FORMAT URI: " + uri)

	return uri, err
}

func (c *Client) decipherURL(ctx context.Context, videoID string, cipher string) (string, error) {
	params, err := url.ParseQuery(cipher)
	if err != nil {
		return "", err
	}

	uri, err := url.Parse(params.Get("url"))
	if err != nil {
		return "", err
	}
	query := uri.Query()

	config, err := c.getPlayerConfig(ctx, videoID)
	if err != nil {
		return "", err
	}

	// decrypt s-parameter
	bs, err := config.decrypt([]byte(params.Get("s")))
	if err != nil {
		return "", err
	}
	query.Add(params.Get("sp"), string(bs))

	// decrypt n-parameter
	nSig := query.Get("n")
	if nSig != "" {
		nDecoded, err := config.decodeNsig(nSig)
		if err != nil {
			return "", fmt.Errorf("unable to decode nSig: %w", err)
		}
		query.Set("n", nDecoded)
	}

	uri.RawQuery = query.Encode()

	return uri.String(), nil
}

func (c *Client) getPlayerConfig(ctx context.Context, videoID string) (playerConfig, error) {

	embedURL := fmt.Sprintf("https://youtube.com/embed/%s?hl=en", videoID)
	embedBody, err := c.httpGetBodyBytes(ctx, embedURL)
	if err != nil {
		return nil, err
	}

	// example: /s/player/f676c671/player_ias.vflset/en_US/base.js
	escapedBasejsURL := string(basejsPattern.Find(embedBody))
	if escapedBasejsURL == "" {
		return nil, errors.New("unable to find basejs URL in playerConfig")
	}

	config := c.playerCache.Get(escapedBasejsURL)
	if config != nil {
		return config, nil
	}

	config, err = c.httpGetBodyBytes(ctx, "https://youtube.com"+escapedBasejsURL)
	if err != nil {
		return nil, err
	}

	c.playerCache.Set(escapedBasejsURL, config)
	return config, nil
}

func (config playerConfig) decrypt(cyphertext []byte) ([]byte, error) {
	operations, err := config.parseDecipherOps()
	if err != nil {
		return nil, err
	}

	// apply operations
	bs := []byte(cyphertext)
	for _, op := range operations {
		bs = op(bs)
	}

	return bs, nil
}

func (config playerConfig) parseDecipherOps() (operations []DecipherOperation, err error) {
	objResult := actionsObjRegexp.FindSubmatch(config)
	funcResult := actionsFuncRegexp.FindSubmatch(config)
	if len(objResult) < 3 || len(funcResult) < 2 {
		return nil, fmt.Errorf("error parsing signature tokens (#obj=%d, #func=%d)", len(objResult), len(funcResult))
	}

	obj := objResult[1]
	objBody := objResult[2]
	funcBody := funcResult[1]

	var reverseKey, spliceKey, swapKey string

	if result := reverseRegexp.FindSubmatch(objBody); len(result) > 1 {
		reverseKey = string(result[1])
	}
	if result := spliceRegexp.FindSubmatch(objBody); len(result) > 1 {
		spliceKey = string(result[1])
	}
	if result := swapRegexp.FindSubmatch(objBody); len(result) > 1 {
		swapKey = string(result[1])
	}

	regex, err := regexp.Compile(fmt.Sprintf("(?:a=)?%s\\.(%s|%s|%s)\\(a,(\\d+)\\)", regexp.QuoteMeta(string(obj)), regexp.QuoteMeta(reverseKey), regexp.QuoteMeta(spliceKey), regexp.QuoteMeta(swapKey)))
	if err != nil {
		return nil, err
	}

	var ops []DecipherOperation
	for _, s := range regex.FindAllSubmatch(funcBody, -1) {
		switch string(s[1]) {
		case reverseKey:
			ops = append(ops, reverseFunc)
		case swapKey:
			arg, _ := strconv.Atoi(string(s[2]))
			ops = append(ops, newSwapFunc(arg))
		case spliceKey:
			arg, _ := strconv.Atoi(string(s[2]))
			ops = append(ops, newSpliceFunc(arg))
		}
	}
	return ops, nil
}

func (config playerConfig) decodeNsig(encoded string) (string, error) {
	fBody, err := config.getNFunction()
	if err != nil {
		return "", err
	}

	return evalJavascript(fBody, encoded)
}

func (config playerConfig) getNFunction() (string, error) {
	nameResult := nFunctionNameRegexp.FindSubmatch(config)
	if len(nameResult) == 0 {
		return "", errors.New("unable to extract n-function name")
	}

	var name string
	if idx, _ := strconv.Atoi(string(nameResult[2])); idx == 0 {
		name = string(nameResult[4])
	} else {
		name = string(nameResult[1])
	}

	// find the beginning of the function
	def := []byte(name + "=function(")
	start := bytes.Index(config, def)
	if start < 1 {
		return "", fmt.Errorf("unable to extract n-function body: looking for '%s'", def)
	}

	// start after the first curly bracket
	pos := start + bytes.IndexByte(config[start:], '{') + 1

	// find the bracket closing the function
	for brackets := 1; brackets > 0; pos++ {
		switch config[pos] {
		case '{':
			brackets++
		case '}':
			brackets--
		}
	}

	return string(config[start:pos]), nil
}

func evalJavascript(jsFunction, arg string) (string, error) {
	const myName = "myFunction"

	vm := goja.New()
	_, err := vm.RunString(myName + "=" + jsFunction)
	if err != nil {
		return "", err
	}

	var output func(string) string
	err = vm.ExportTo(vm.Get(myName), &output)
	if err != nil {
		return "", err
	}

	return output(arg), nil
}

func (c *Client) httpGetBodyBytes(ctx context.Context, url string) ([]byte, error) {
	resp, err := c.httpGet(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *Client) httpGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpDo(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, ErrUnexpectedStatusCode(resp.StatusCode)
	}
	return resp, nil
}

// httpDo sends an HTTP request and returns an HTTP response.
func (c *Client) httpDo(req *http.Request) (*http.Response, error) {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	if c.Debug {
		log.Println(req.Method, req.URL)
	}

	res, err := client.Do(req)

	if c.Debug && res != nil {
		log.Println(res.Status)
	}

	return res, err
}

// Get : get cache  when it has same video id and not expired
func (s playerCache) Get(key string) playerConfig {
	return s.GetCacheBefore(key, time.Now())
}

// GetCacheBefore : can pass time for testing
func (s playerCache) GetCacheBefore(key string, time time.Time) playerConfig {
	if key == s.key && s.expiredAt.After(time) {
		return s.config
	}
	return nil
}

// Set : set cache with default expiration
func (s *playerCache) Set(key string, operations playerConfig) {
	s.setWithExpiredTime(key, operations, time.Now().Add(defaultCacheExpiration))
}

func (s *playerCache) setWithExpiredTime(key string, config playerConfig, time time.Time) {
	s.key = key
	s.config = config
	s.expiredAt = time
}

func newSpliceFunc(pos int) DecipherOperation {
	return func(bs []byte) []byte {
		return bs[pos:]
	}
}

func newSwapFunc(arg int) DecipherOperation {
	return func(bs []byte) []byte {
		pos := arg % len(bs)
		bs[0], bs[pos] = bs[pos], bs[0]
		return bs
	}
}

func reverseFunc(bs []byte) []byte {
	l, r := 0, len(bs)-1
	for l < r {
		bs[l], bs[r] = bs[r], bs[l]
		l++
		r--
	}
	return bs
}
