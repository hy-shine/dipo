package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hy-shine/dipo/internal/colour"
)

var errSamplerProtocol = errors.New("invalid sampler result")

type sample struct {
	Status string  `json:"status"`
	Red    float64 `json:"red"`
	Green  float64 `json:"green"`
	Blue   float64 `json:"blue"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

// measured is the picked color as the screen showed it, in extended sRGB.
func (picked sample) measured() colour.SRGB {
	return colour.SRGB{R: picked.Red, G: picked.Green, B: picked.Blue}
}

func sampleScreen(ctx context.Context, hint string) (sample, error) {
	helper, err := samplerHelper()
	if err != nil {
		return sample{}, err
	}
	output, err := runSampler(ctx, helper, "--hint", hint)
	if err != nil {
		return sample{}, err
	}
	return decodeSample(output)
}

func samplerHelper() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate application: %w", err)
	}
	return filepath.Join(filepath.Dir(executable), "..", "Resources", "dipo-sampler"), nil
}

func runSampler(ctx context.Context, helper string, args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, helper, args...)
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("run sampler: %w", ctx.Err())
		}
		return nil, fmt.Errorf("run sampler (%s): %w", bytes.TrimSpace(stderr.Bytes()), err)
	}
	return output, nil
}

// screenCapturePermission checks the helper's current TCC state without
// prompting for access. The helper only requests permission after the user
// starts a pick.
func screenCapturePermission(ctx context.Context) (bool, error) {
	helper, err := samplerHelper()
	if err != nil {
		return false, err
	}
	output, err := runSampler(ctx, helper, "--check-permission")
	if err != nil {
		return false, err
	}
	return decodePermissionStatus(output)
}

func decodePermissionStatus(data []byte) (bool, error) {
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return false, fmt.Errorf("%w: %v", errSamplerProtocol, err)
	}
	switch result.Status {
	case "ready":
		return true, nil
	case "permission":
		return false, nil
	default:
		return false, fmt.Errorf("%w: unknown permission status %q", errSamplerProtocol, result.Status)
	}
}

func decodeSample(data []byte) (sample, error) {
	var result struct {
		Status string   `json:"status"`
		Red    *float64 `json:"red"`
		Green  *float64 `json:"green"`
		Blue   *float64 `json:"blue"`
		X      *float64 `json:"x"`
		Y      *float64 `json:"y"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return sample{}, fmt.Errorf("%w: %v", errSamplerProtocol, err)
	}
	if result.Status == "cancelled" || result.Status == "permission" {
		return sample{Status: result.Status}, nil
	}
	if result.Status != "picked" {
		return sample{}, fmt.Errorf("%w: unknown status %q", errSamplerProtocol, result.Status)
	}
	for _, component := range []*float64{result.Red, result.Green, result.Blue, result.X, result.Y} {
		if component == nil || math.IsNaN(*component) || math.IsInf(*component, 0) {
			return sample{}, fmt.Errorf("%w: missing or non-finite color or position", errSamplerProtocol)
		}
	}
	return sample{Status: result.Status, Red: *result.Red, Green: *result.Green, Blue: *result.Blue, X: *result.X, Y: *result.Y}, nil
}
