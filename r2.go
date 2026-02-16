package main

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type R2Client struct {
	Client        *s3.Client
	Bucket        string
	PublicBaseURL string
}

func NewR2Client(cfg Config) (*R2Client, error) {
	if cfg.R2Endpoint == "" {
		return nil, errors.New("R2_ENDPOINT missing")
	}
	if cfg.R2AccessKey == "" || cfg.R2SecretKey == "" {
		return nil, errors.New("R2 access keys missing")
	}
	if cfg.R2Bucket == "" {
		return nil, errors.New("R2_BUCKET missing")
	}
	if cfg.R2PublicBaseURL == "" {
		return nil, errors.New("R2_PUBLIC_BASE_URL missing")
	}

	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if service == s3.ServiceID {
			return aws.Endpoint{
				URL:               cfg.R2Endpoint,
				HostnameImmutable: true,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	awsCfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(cfg.R2Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.R2AccessKey, cfg.R2SecretKey, "")),
		config.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = true
	})

	return &R2Client{
		Client:        client,
		Bucket:        cfg.R2Bucket,
		PublicBaseURL: cfg.R2PublicBaseURL,
	}, nil
}

func (r *R2Client) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	_, err := r.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}

	publicURL, err := joinPublicURL(r.PublicBaseURL, key)
	if err != nil {
		return "", err
	}
	return publicURL, nil
}

func joinPublicURL(base string, key string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	parsed.Path = path.Join(parsed.Path, key)
	return parsed.String(), nil
}
