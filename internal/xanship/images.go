package xanship

import "strings"

type ImageTransferMode string

const (
	ImageTransferPull     ImageTransferMode = "pull"
	ImageTransferSaveLoad ImageTransferMode = "save-load"
)

type ImageTransferDecision struct {
	Image  string            `json:"image"`
	Mode   ImageTransferMode `json:"mode"`
	Reason string            `json:"reason"`
}

func DecideImageTransfer(image string) ImageTransferDecision {
	first := image
	if i := strings.IndexByte(image, '/'); i >= 0 {
		first = image[:i]
	}
	if image == "" {
		return ImageTransferDecision{Image: image, Mode: ImageTransferSaveLoad, Reason: "empty image reference cannot be pulled"}
	}
	if !strings.Contains(first, ".") && !strings.Contains(first, ":") && first != "localhost" {
		return ImageTransferDecision{Image: image, Mode: ImageTransferPull, Reason: "public Docker Hub style reference"}
	}
	return ImageTransferDecision{Image: image, Mode: ImageTransferPull, Reason: "registry reference; falls back to docker save/load if pull requires credentials or fails"}
}
