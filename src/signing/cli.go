package signing

import (
	"LocalSignTools/src/builders"
	"LocalSignTools/src/config"
	"LocalSignTools/src/storage"
	"bufio"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CLISigningOptions contains options for CLI mode signing
type CLISigningOptions struct {
	IPAFile      string
	ProfileName  string
	OutputPath   string
	SignArgs     string
	UserBundleID string
	BuilderID    string
}

// RunCLISigning performs signing in CLI mode (synchronous)
func RunCLISigning(opts CLISigningOptions) error {
	// Load storage
	storage.Load()

	// Get profile
	profile, ok := storage.Profiles.GetById(opts.ProfileName)
	if !ok {
		return errors.Errorf("profile not found: %s", opts.ProfileName)
	}

	// Get builder
	if opts.BuilderID == "" {
		// Use the first available builder (usually "Integrated")
		for id := range config.Current.Builder {
			opts.BuilderID = id
			break
		}
	}
	builder, ok := config.Current.Builder[opts.BuilderID]
	if !ok {
		return errors.Errorf("builder not found: %s", opts.BuilderID)
	}

	// Open IPA file
	ipaFile, err := os.Open(opts.IPAFile)
	if err != nil {
		return errors.WithMessage(err, "open IPA file")
	}
	defer ipaFile.Close()

	// Get file name from path
	fileName := filepath.Base(opts.IPAFile)

	// Create app in storage
	app, err := storage.Apps.New(
		ipaFile,
		fileName,
		profile,
		opts.SignArgs,
		opts.UserBundleID,
		opts.BuilderID,
		map[string]io.Reader{}, // No tweaks in CLI mode for now
	)
	if err != nil {
		return errors.WithMessage(err, "create app")
	}

	log.Info().
		Str("app_id", app.GetId()).
		Str("profile", opts.ProfileName).
		Str("ipa", opts.IPAFile).
		Msg("starting CLI signing")

	// Perform synchronous signing
	if err := performSynchronousSigning(app, builder); err != nil {
		return errors.WithMessage(err, "signing failed")
	}

	// Check if signed file exists
	signed, err := app.IsSigned()
	if err != nil {
		return errors.WithMessage(err, "check signed status")
	}
	if !signed {
		return errors.New("signing completed but signed file not found")
	}

	// Copy signed file to output path
	signedFile, err := app.GetFile(storage.AppSignedFile)
	if err != nil {
		return errors.WithMessage(err, "get signed file")
	}
	defer signedFile.Close()

	outputFile, err := os.Create(opts.OutputPath)
	if err != nil {
		return errors.WithMessage(err, "create output file")
	}
	defer outputFile.Close()

	if _, err := io.Copy(outputFile, signedFile); err != nil {
		return errors.WithMessage(err, "copy signed file to output")
	}

	log.Info().
		Str("app_id", app.GetId()).
		Str("output", opts.OutputPath).
		Msg("CLI signing completed successfully")

	return nil
}

// performSynchronousSigning performs signing synchronously (waits for completion)
func performSynchronousSigning(app storage.App, builder builders.Builder) error {
	profileId, err := app.GetString(storage.AppProfileId)
	if err != nil {
		return errors.WithMessage(err, "get profile id")
	}

	appId := app.GetId()
	
	// Create a job for tracking
	storage.Jobs.MakeSignJob(appId, profileId)
	log.Info().Str("app_id", appId).Msg("created signing job")

	// Set builder secrets
	if err := config.SetBuilderSecrets(builder); err != nil {
		return errors.WithMessage(err, "set builder secrets")
	}

	// For integrated builder, we need to process the job directly
	if integrated, ok := builder.(*builders.Integrated); ok {
		return processIntegratedJobDirectly(app, integrated, appId)
	}

	// For other builders, trigger and wait (not implemented for now)
	return errors.New("only integrated builder is supported in CLI mode")
}

// setBuilderSecrets is now in config.SetBuilderSecrets

// processIntegratedJobDirectly processes an integrated builder job synchronously
func processIntegratedJobDirectly(app storage.App, integrated *builders.Integrated, appId string) error {
	// Setup integrated builder with process function
	jobAdapter := &storage.JobStorageAdapter{}
	appAdapter := &storage.AppStorageAdapter{}
	
	processFn := func() error {
		return builders.ProcessIntegratedJob(integrated, jobAdapter, appAdapter, storage.ErrNotFound)
	}
	
	// Set the process function (this will start the worker if not already started)
	integrated.SetProcessJobFn(processFn)

	// Trigger the builder to start processing
	if err := integrated.Trigger(); err != nil {
		return errors.WithMessage(err, "trigger builder")
	}

	log.Info().Str("app_id", appId).Msg("triggered builder, waiting for completion")

	// Wait for the job to be processed
	maxWaitTime := integrated.GetJobTimeout()
	pollInterval := 500 * time.Millisecond
	checkInterval := 2 * time.Second // Check for 2FA prompt every 2 seconds
	startTime := time.Now()
	var returnJobId string
	last2FACheck := time.Now()
	twoFAMessageShown := false // Track if 2FA message has already been shown

	for time.Since(startTime) < maxWaitTime {
		// Check if return job exists (created when TakeLastJob is called)
		if job, ok := storage.Jobs.GetByAppId(appId); ok {
			if returnJobId == "" {
				returnJobId = job.Id
				log.Info().Str("return_job_id", returnJobId).Msg("return job created, waiting for processing")
			}
			
			// Check if 2FA is needed (periodically check)
			if time.Since(last2FACheck) >= checkInterval {
				twoFactorCode := job.TwoFactorCode.Load()
				if twoFactorCode == "" {
					// Show 2FA message only once after a short delay
					if !twoFAMessageShown && time.Since(startTime) > 5*time.Second {
						log.Info().Msg("If 2FA code is required, you will be prompted by fastlane.")
						twoFAMessageShown = true
					}
				} else {
					if !twoFAMessageShown {
						log.Info().Str("return_job_id", returnJobId).Msg("2FA code provided, waiting for processing")
						twoFAMessageShown = true
					}
				}
				last2FACheck = time.Now()
			}
		} else {
			// Return job was deleted, check if app is signed
			signed, err := app.IsSigned()
			if err != nil {
				return errors.WithMessage(err, "check signed status")
			}
			if signed {
				log.Info().Str("app_id", appId).Msg("job completed successfully")
				return nil
			}
			// Job might have failed, check pending status
			pending, exists := storage.Jobs.GetStatusByAppId(appId)
			if !pending && !exists {
				return errors.New("job completed but app is not signed")
			}
		}

		time.Sleep(pollInterval)
	}

	return errors.New("timeout waiting for job to be processed")
}

// Prompt2FA prompts for 2FA code from stdin and sets it in the job
func Prompt2FA(appId string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter 2FA code: ")
	code, err := reader.ReadString('\n')
	if err != nil {
		return "", errors.WithMessage(err, "read 2FA code")
	}
	// Trim whitespace
	code = strings.TrimSpace(code)
	
	// Set 2FA code in the job
	if job, ok := storage.Jobs.GetByAppId(appId); ok {
		job.TwoFactorCode.Store(code)
		log.Info().Str("app_id", appId).Msg("2FA code set in job")
	}
	
	return code, nil
}
