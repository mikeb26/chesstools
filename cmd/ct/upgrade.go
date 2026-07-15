package main

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mikeb26/chesstools"
)

const ProjName = "chesstools"
const CliName = "ct"
const BrewVersionSuffix = "b"
const DevVersionText = "v0.devbuild"

//go:embed version.txt
var versionText string

func versionMain(args []string) {
	fmt.Printf("chesstools-%v\n", versionText)
}

func upgradeMain(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			printUpgradeUsage()
			return
		}
	}

	if err := upgradeMainWork(); err != nil {
		fmt.Fprintf(os.Stderr, "%v upgrade failed: %v\n", CliName, err)
		os.Exit(1)
	}
}

func printUpgradeUsage() {
	fmt.Fprintf(os.Stdout, "usage: %v upgrade\n", CliName)
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "upgrade the %v binary to the latest GitHub release\n", ProjName)
}

func upgradeMainWork() error {
	if versionText == DevVersionText {
		fmt.Fprintf(os.Stderr, "Skipping %v upgrade on development version\n", ProjName)
		return nil
	}

	latestVer, err := getLatestVersion()
	if err != nil {
		return err
	}
	if latestVer == versionText {
		fmt.Printf("%v %v is already the latest version\n", ProjName, versionText)
		return nil
	}

	fmt.Printf("A new version of %v is available (%v). Upgrade? (Y/N) [Y]: ",
		ProjName, latestVer)
	shouldUpgrade, err := readUpgradeConfirmation(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}
	if !shouldUpgrade {
		return nil
	}

	fmt.Printf("Upgrading %v from %v to %v...\n", ProjName, versionText, latestVer)

	if isBrewVersion() {
		err = upgradeCLIViaBrew()
	} else {
		err = upgradeViaGithub(latestVer)
	}
	return err
}

func readUpgradeConfirmation(r io.Reader) (bool, error) {
	confirmation, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}

	confirmation = strings.ToUpper(strings.TrimSpace(confirmation))
	if confirmation == "" {
		return true, nil
	}

	return confirmation[0] == 'Y', nil
}

func getLatestVersion() (string, error) {
	const latestReleaseURL = "https://api.github.com/repos/mikeb26/chesstools/releases/latest"

	req, err := http.NewRequest("GET", latestReleaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", chesstools.UserAgent)

	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("no GitHub releases published for %v yet", ProjName)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return "", fmt.Errorf("failed to fetch latest release: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("could not parse %s", latestReleaseURL)
	}

	if isBrewVersion() {
		release.TagName += BrewVersionSuffix
	}

	return release.TagName, nil
}

func upgradeViaGithub(latestVer string) error {
	const latestDownloadFmt = "https://github.com/mikeb26/chesstools/releases/download/%v/ct"

	req, err := http.NewRequest("GET", fmt.Sprintf(latestDownloadFmt, latestVer), nil)
	if err != nil {
		return fmt.Errorf("failed to prepare download request: %w", err)
	}
	req.Header.Set("User-Agent", chesstools.UserAgent)

	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download version %v: %w", versionText, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return fmt.Errorf("failed to download version %v: %s: %s", versionText, resp.Status, strings.TrimSpace(string(body)))
	}

	tmpFile, err := os.CreateTemp("", "ct-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	binaryContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to download version %v: %w", versionText, err)
	}
	if _, err := tmpFile.Write(binaryContent); err != nil {
		return fmt.Errorf("failed to download version %v: %w", versionText, err)
	}
	if err := tmpFile.Chmod(0o755); err != nil {
		return fmt.Errorf("failed to download version %v: %w", versionText, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to download version %v: %w", versionText, err)
	}

	myBinaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine path to %v: %w", CliName, err)
	}
	myBinaryPath, err = filepath.EvalSymlinks(myBinaryPath)
	if err != nil {
		return fmt.Errorf("could not determine path to %v: %w", CliName, err)
	}

	myBinaryPathBak := myBinaryPath + ".bak"
	if err := os.Rename(myBinaryPath, myBinaryPathBak); err != nil {
		return fmt.Errorf("could not replace existing %v; do you need to be root?: %w", myBinaryPath, err)
	}

	err = os.Rename(tmpFile.Name(), myBinaryPath)
	if errors.Is(err, syscall.EXDEV) {
		if err := os.WriteFile(myBinaryPath, binaryContent, 0o755); err != nil {
			_ = os.Rename(myBinaryPathBak, myBinaryPath)
			return fmt.Errorf("could not replace existing %v; do you need to be root?: %w", myBinaryPath, err)
		}
		err = nil
	}
	if err != nil {
		restoreErr := os.Rename(myBinaryPathBak, myBinaryPath)
		if restoreErr != nil {
			return fmt.Errorf("could not replace existing %v; do you need to be root?: %w (and restore failed: %v)", myBinaryPath, err, restoreErr)
		}
		return fmt.Errorf("could not replace existing %v; do you need to be root?: %w", myBinaryPath, err)
	}
	_ = os.Remove(myBinaryPathBak)

	fmt.Printf("Upgrade %v to %v complete\n", myBinaryPath, latestVer)

	return nil
}

func checkAndPrintUpgradeWarning() bool {
	if versionText == DevVersionText {
		return false
	}
	latestVer, err := getLatestVersion()
	if err != nil {
		return false
	}
	if latestVer == versionText {
		return false
	}

	fmt.Fprintf(os.Stderr, "*WARN*: A new version of %v is available (%v). Please upgrade via '%v upgrade'.\n\n",
		ProjName, latestVer, CliName)

	return true
}

func isBrewVersion() bool {
	if versionText[len(versionText)-1] == BrewVersionSuffix[0] {
		return true
	}

	return false
}

func upgradeCLIViaBrew() error {
	ctx := context.Background()
	err := runHostCommand(ctx, []string{"brew", "update"}, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to update brew formulae: %w\n", err)
	}
	err = runHostCommand(ctx, []string{"brew", "install", "mikeb26/tap/chesstools"}, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to upgrade %v: %w\n", ProjName, err)
	}

	return nil
}

func runHostCommand(ctx context.Context, cmdAndArgs []string, stdOut io.Writer, stdErr io.Writer) error {
	cmd := exec.CommandContext(ctx, cmdAndArgs[0], cmdAndArgs[1:]...)
	cmd.Stderr = stdErr
	cmd.Stdout = stdOut
	return cmd.Run()
}
