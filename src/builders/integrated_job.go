package builders

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	dirCopy "github.com/otiai10/copy"
	"LocalSignTools/src/util"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// JobStorage defines the interface for job storage operations
// This allows ProcessIntegratedJob to work without directly importing storage package
type JobStorage interface {
	TakeLastJob(writer io.Writer) error
	GetById(id string) (ReturnJob, bool)
	DeleteById(id string) bool
}

// ReturnJob defines the interface for return job operations
type ReturnJob interface {
	GetAppId() string
}

// AppStorage defines the interface for app storage operations
type AppStorage interface {
	Get(id string) (App, bool)
}

// App defines the interface for app operations
type App interface {
	GetFile(name string) (io.ReadCloser, error)
	SetFile(name string, file io.ReadSeeker) error
	SetString(name string, value string) error
}

// extractJobIdFromArchive extracts the job ID from a tar archive buffer
func extractJobIdFromArchive(archiveBuffer *bytes.Buffer) string {
	if archiveBuffer.Len() == 0 {
		return ""
	}
	tr := tar.NewReader(bytes.NewReader(archiveBuffer.Bytes()))
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		if header.Name == "id.txt" {
			idBuf := make([]byte, header.Size)
			if _, err := io.ReadFull(tr, idBuf); err == nil {
				return util.TrimWhitespace(string(idBuf))
			}
			break
		}
		// Skip other files
		if header.Size > 0 {
			io.Copy(io.Discard, tr)
		}
	}
	return ""
}

// ProcessIntegratedJob processes a job for the integrated builder
// This function is called from the integrated builder's worker goroutine
// Dependencies are injected to avoid circular imports
func ProcessIntegratedJob(integrated *Integrated, jobStorage JobStorage, appStorage AppStorage, errNotFound error) error {
	// Get the last job from storage and read archive into memory
	var archiveBuffer bytes.Buffer
	var archiveErr error
	
	// Create a pipe to read the job archive
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		if err := jobStorage.TakeLastJob(pw); err != nil {
			if errors.Is(err, errNotFound) {
				log.Debug().Msg("no job found for integrated builder")
				archiveErr = err
				return
			}
			log.Error().Err(err).Msg("take last job")
			archiveErr = err
			return
		}
	}()

	// Read entire archive into buffer
	if _, err := io.Copy(&archiveBuffer, pr); err != nil {
		log.Error().Err(err).Msg("read job archive")
		pr.Close()
		// Try to extract job ID even if archive read failed partially
		// This allows us to clean up the job even if there was an error
		if returnJobId := extractJobIdFromArchive(&archiveBuffer); returnJobId != "" {
			if jobStorage.DeleteById(returnJobId) {
				log.Info().Str("job_id", returnJobId).Msg("cleaned up job after archive read error")
			}
		}
		return err
	}
	pr.Close()

	if archiveErr != nil {
		if errors.Is(archiveErr, errNotFound) {
			return nil // No job available, not an error
		}
		return archiveErr
	}

	// Extract job ID from archive
	returnJobId := extractJobIdFromArchive(&archiveBuffer)
	if returnJobId == "" {
		return errors.New("job id not found in archive")
	}

	// Get return job to find app ID
	returnJob, ok := jobStorage.GetById(returnJobId)
	if !ok {
		return errors.Errorf("return job not found: %s", returnJobId)
	}
	appId := returnJob.GetAppId()

	// Use the archive buffer we already read
	jobReader := bytes.NewReader(archiveBuffer.Bytes())

	// Process the job
	id := fmt.Sprintf("integrated-%d", time.Now().UnixNano())
	log.Info().Str("job_id", id).Str("app_id", appId).Msg("running integrated sign job")

	ctx, cancel := context.WithTimeout(context.Background(), integrated.GetJobTimeout())
	defer cancel()

	err := func() error {
		tempDir, err := os.MkdirTemp("", "ios-signer-integrated-")
		if err != nil {
			return errors.WithMessage(err, "make temp dir")
		}
		defer os.RemoveAll(tempDir)

		workDir, err := filepath.Abs(tempDir)
		if err != nil {
			return errors.WithMessage(err, "get work dir absolute path")
		}

		// Copy sign files
		if err := dirCopy.Copy(integrated.GetSignFilesDir(), workDir); err != nil {
			return errors.WithMessage(err, "copy sign files")
		}

		// Extract job archive
		tr := tar.NewReader(jobReader)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return errors.WithMessage(err, "read tar")
			}

			targetPath := filepath.Join(workDir, header.Name)
			if header.Typeflag == tar.TypeDir {
				if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
					return errors.WithMessagef(err, "mkdir %s", header.Name)
				}
				continue
			}

			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return errors.WithMessagef(err, "mkdir parent %s", targetPath)
			}

			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return errors.WithMessagef(err, "create file %s", header.Name)
			}
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return errors.WithMessagef(err, "write file %s", header.Name)
			}
			file.Close()
		}

		// Copy unsigned.ipa to work directory (for integrated builder)
		app, ok := appStorage.Get(appId)
		if !ok {
			return errors.Errorf("app not found: %s", appId)
		}
		unsignedFile, err := app.GetFile("unsigned")
		if err != nil {
			return errors.WithMessage(err, "get unsigned file")
		}
		defer unsignedFile.Close()
		
		unsignedPath := filepath.Join(workDir, "unsigned.ipa")
		unsignedOut, err := os.Create(unsignedPath)
		if err != nil {
			return errors.WithMessage(err, "create unsigned.ipa")
		}
		defer unsignedOut.Close()
		if _, err := io.Copy(unsignedOut, unsignedFile); err != nil {
			return errors.WithMessage(err, "copy unsigned.ipa")
		}

		// Prepare environment
		signEnv := os.Environ()
		secretsMap := integrated.GetSecrets()
		for key, val := range secretsMap {
			signEnv = append(signEnv, key+"="+val)
		}
		signEnv = append(signEnv, "PYTHONUNBUFFERED=1")
		// Set flag to indicate integrated builder mode (job archive already extracted)
		signEnv = append(signEnv, "INTEGRATED_BUILDER=1")

		// Execute sign script
		entrypointPath := filepath.Join(workDir, integrated.GetEntrypoint())
		cmd := exec.CommandContext(ctx, entrypointPath)
		cmd.Dir = workDir
		cmd.Env = signEnv

		// Stream output in real-time for better debugging (especially for 2FA)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return errors.WithMessage(err, "create stdout pipe")
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return errors.WithMessage(err, "create stderr pipe")
		}

		if err := cmd.Start(); err != nil {
			return errors.WithMessage(err, "start sign script")
		}

		// Stream stdout and stderr to logs in real-time
		var outputBuffer bytes.Buffer
		outputDone := make(chan bool)
		go func() {
			defer close(outputDone)
			multiWriter := io.MultiWriter(&outputBuffer, os.Stdout)
			
			// Read from stdout and stderr concurrently
			stdoutDone := make(chan bool)
			stderrDone := make(chan bool)
			
			// Read stdout
			go func() {
				defer func() { stdoutDone <- true }()
				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					line := scanner.Text()
					multiWriter.Write([]byte(line + "\n"))
					// Log only truly important messages (errors, critical warnings, 2FA prompts)
					// Skip routine fastlane output to reduce log noise
					lineLower := strings.ToLower(line)
					if strings.Contains(lineLower, "error") ||
						strings.Contains(lineLower, "failed") ||
						strings.Contains(lineLower, "exception") ||
						strings.Contains(lineLower, "two-factor authentication (2fa) code") ||
						strings.Contains(lineLower, "please enter") ||
						(strings.Contains(lineLower, "2fa") && (strings.Contains(lineLower, "code") || strings.Contains(lineLower, "required"))) {
						log.Info().Str("line", line).Msg("sign script")
					}
				}
			}()
			
			// Read stderr
			go func() {
				defer func() { stderrDone <- true }()
				scanner := bufio.NewScanner(stderr)
				for scanner.Scan() {
					line := scanner.Text()
					multiWriter.Write([]byte(line + "\n"))
					// Log all stderr messages (usually errors)
					log.Warn().Str("line", line).Msg("sign script stderr")
				}
			}()
			
			<-stdoutDone
			<-stderrDone
		}()

		err = cmd.Wait()
		<-outputDone

		if err != nil {
			output := outputBuffer.String()
			log.Error().Err(err).Str("output", output).Msg("sign script failed")
			return errors.WithMessage(errors.WithMessage(errors.New(output), err.Error()), "sign script")
		}

		// Read signed app
		signedPath := filepath.Join(workDir, "signed.ipa")
		if _, err := os.Stat(signedPath); err != nil {
			return errors.WithMessage(err, "signed app not found")
		}

		// Upload signed app back to storage
		signedFile, err := os.Open(signedPath)
		if err != nil {
			return errors.WithMessage(err, "open signed app")
		}
		defer signedFile.Close()

		// app is already defined above, reuse it
		if err := app.SetFile("signed", signedFile); err != nil {
			return errors.WithMessage(err, "set signed file")
		}

		// Read bundle ID if available
		bundleIdPath := filepath.Join(workDir, "bundle_id.txt")
		if bundleIdBytes, err := os.ReadFile(bundleIdPath); err == nil {
			bundleId := util.TrimWhitespace(string(bundleIdBytes))
			if err := app.SetString("bundle_id", bundleId); err != nil {
				log.Warn().Err(err).Msg("set bundle id")
			}
		}

		// Clean up return job
		if !jobStorage.DeleteById(returnJobId) {
			log.Warn().Str("job_id", returnJobId).Msg("unable to delete return job")
		}

		return nil
	}()

	if err != nil {
		log.Error().Err(err).Str("job_id", id).Msg("integrated sign job failed")
		// Mark job as failed
		if !jobStorage.DeleteById(returnJobId) {
			log.Warn().Str("job_id", returnJobId).Msg("unable to delete failed return job")
		}
		return err
	}

	log.Info().Str("job_id", id).Msg("integrated sign job completed")
	return nil
}
