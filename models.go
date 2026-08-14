package main

type Config struct {
	RegistryID     string `json:"registryId"`
	RegistryURL    string `json:"registryUrl"`
	ProxyMode      string `json:"proxyMode"`
	ProxyURL       string `json:"proxyUrl"`
	AutoOpen       bool   `json:"autoOpen"`
	AutoStart      bool   `json:"autoStart"`
	InstallChannel string `json:"installChannel"`
}

type RegistryPreset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Recommended bool   `json:"recommended"`
}

type RegistryResult struct {
	ID        string `json:"id"`
	OK        bool   `json:"ok"`
	LatencyMS int64  `json:"latencyMs"`
	Message   string `json:"message"`
}

type JobState struct {
	Type       string  `json:"type"`
	Phase      string  `json:"phase"`
	Title      string  `json:"title"`
	Message    string  `json:"message"`
	Progress   float64 `json:"progress"`
	StartedAt  int64   `json:"startedAt"`
	FinishedAt int64   `json:"finishedAt"`
}

type LogEntry struct {
	ID     int64  `json:"id"`
	Time   string `json:"time"`
	Source string `json:"source"`
	Level  string `json:"level"`
	Text   string `json:"text"`
}

type Status struct {
	Installed       bool             `json:"installed"`
	Version         string           `json:"version"`
	NodeReady       bool             `json:"nodeReady"`
	NodeVersion     string           `json:"nodeVersion"`
	Service         string           `json:"service"`
	ServicePID      int              `json:"servicePid"`
	Platform        string           `json:"platform"`
	Architecture    string           `json:"architecture"`
	InstallPath     string           `json:"installPath"`
	ServiceURL      string           `json:"serviceUrl"`
	Job             JobState         `json:"job"`
	Config          Config           `json:"config"`
	Registries      []RegistryPreset `json:"registries"`
	Logs            []LogEntry       `json:"logs"`
	DownloadSupport bool             `json:"downloadSupport"`
}
