package defs

const (
	Port = 50000
)

type InitialJson struct {
	Dir  string
	Ftar string

	VR      bool   `json:"vr,omitempty"`
	HairSeg bool   `json:"hair_seg,omitempty"`
	FPS     int    `json:"fps,omitempty"`
	W       int    `json:"width,omitempty"`
	H       int    `json:"height,omitempty"`
	Static  string `json:"static,omitempty"`
	Bkg     bool   `json:"background,omitempty"`
	Batch_s int    `json:"batch_size,omitempty"`
	Tattoo  int    `json:"tattoo,omitempty"`
	Blur    bool   `json:"motion_blur,omitempty"`
	Glasses bool   `json:"glasses,omitempty"`
	Hat     bool   `json:"hat,omitempty"`
	Mask    int    `json:"merge_type,omitempty"`
	Color   int    `json:"color_filter,omitempty"`
	Pi      int    `json:"pattern_index,omitempty"`
}

type Anim struct {
	Ts int `json:"ts"` // milliseconds since start

	Audio   string    `json:"audio,omitempty"`   // file name with audio samples
	Phones  []*Viseme `json:"phones,omitempty"`  // phones like derived from vosk
	Pattern []float64 `json:"pattern,omitempty"` // model params
}

type Viseme struct {
	Time     int    `json:"time"` // ms
	Type     string `json:"type"`
	start    int
	Value    string `json:"value"`
	end      int
	Duration int `json:"duration"` // ms
}
