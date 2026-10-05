package credproxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
)

// SigV4Auth signs upstream requests with AWS Signature V4, using credentials
// resolved on the host (named profile, SSO, env, IMDS…). The tool never sees
// any AWS credential: it only holds the proxy session token.
type SigV4Auth struct {
	Credentials aws.CredentialsProvider
	Region      string
	Service     string           // "bedrock" for Bedrock runtime
	Now         func() time.Time // test hook (default time.Now)

	once   sync.Once
	signer *v4.Signer
}

// NewSigV4FromProfile loads AWS credentials for profile (empty = default chain).
func NewSigV4FromProfile(ctx context.Context, profile, region string) (*SigV4Auth, error) {
	opts := []func(*config.LoadOptions) error{config.WithRegion(region)}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config (profile %q): %w", profile, err)
	}
	if _, err := cfg.Credentials.Retrieve(ctx); err != nil {
		return nil, fmt.Errorf("resolving AWS credentials (profile %q): %w", profile, err)
	}
	return &SigV4Auth{Credentials: aws.NewCredentialsCache(cfg.Credentials), Region: region, Service: "bedrock"}, nil
}

// AWSRegion is the region the AWS SDK resolves for profile (empty = default
// chain): AWS_REGION / AWS_DEFAULT_REGION, then the shared config profile.
// Empty when none is configured.
func AWSRegion(ctx context.Context, profile string) string {
	var opts []func(*config.LoadOptions) error
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return ""
	}
	return cfg.Region
}

// Apply implements Auth.
func (a *SigV4Auth) Apply(req *http.Request, body []byte) error {
	if a.Credentials == nil {
		return fmt.Errorf("sigv4: no credentials provider")
	}
	creds, err := a.Credentials.Retrieve(req.Context())
	if err != nil {
		return fmt.Errorf("sigv4: retrieving credentials: %w", err)
	}
	// Non-S3 services sign the (already escaped) path escaped once more,
	// which is the SDK default.
	a.once.Do(func() { a.signer = v4.NewSigner() })
	now := time.Now
	if a.Now != nil {
		now = a.Now
	}
	sum := sha256.Sum256(body)
	req.Header.Del("Authorization")
	service := a.Service
	if service == "" {
		service = "bedrock"
	}
	return a.signer.SignHTTP(req.Context(), creds, req, hex.EncodeToString(sum[:]), service, a.Region, now().UTC())
}
