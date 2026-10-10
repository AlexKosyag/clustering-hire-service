// Package storage — MinIO (резюме, аватары, логотипы), docs/07 §4.
// Аналог src/modules/storage/storage.service.ts: upload → публичный URL,
// любой сбой хранилища → 503 ServiceUnavailableException.
package storage

import (
	"bytes"
	"context"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"fsp-backend/internal/common"
	"fsp-backend/internal/config"
)

// Service — клиент MinIO (upload → публичный URL).
type Service struct {
	client     *minio.Client // nil — MinIO не инициализирован (кривой endpoint)
	bucket     string
	publicBase string

	mu        sync.Mutex
	initTried bool // ленивое создание бакета (ensureBucket)
}

// New создаёт сервис файлового хранилища (без подключения — клиент ленивый).
func New(cfg *config.Config) *Service {
	s := &Service{
		bucket: cfg.MinioBucket,
		// MINIO_PUBLIC_ENDPOINT || http://localhost:9000, без завершающего "/".
		publicBase: strings.TrimSuffix(cfg.MinioPublicEndpoint, "/"),
	}
	if s.bucket == "" {
		s.bucket = "fsp-platform"
	}
	if s.publicBase == "" {
		s.publicBase = "http://localhost:9000"
	}

	raw := cfg.MinioEndpoint
	if raw == "" {
		raw = "http://localhost:9000"
	}
	endpoint, useSSL, err := parseEndpoint(raw)
	if err != nil {
		log.Printf("[Storage] MinIO не инициализирован: %s", err.Error())
		return s // client = nil
	}

	accessKey := cfg.MinioAccessKey
	if accessKey == "" {
		accessKey = "minioadmin"
	}
	secretKey := cfg.MinioSecretKey
	if secretKey == "" {
		secretKey = "minioadmin"
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Printf("[Storage] MinIO не инициализирован: %s", err.Error())
		return s
	}
	s.client = client
	return s
}

// parseEndpoint — new URL(raw) из storage.service.ts: hostname + порт
// (url.port || (https ? 443 : 80)) + useSSL.
func parseEndpoint(raw string) (hostPort string, useSSL bool, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", false, err
	}
	host := u.Hostname()
	if host == "" {
		return "", false, errMinioEndpoint
	}
	useSSL = u.Scheme == "https"
	port := 80
	if useSSL {
		port = 443
	}
	if p := u.Port(); p != "" {
		if n, convErr := strconv.Atoi(p); convErr == nil {
			port = n
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), useSSL, nil
}

// errMinioEndpoint — endpoint не разбирается в URL с хостом.
var errMinioEndpoint = &endpointError{"некорректный MINIO_ENDPOINT"}

type endpointError struct{ msg string }

func (e *endpointError) Error() string { return e.msg }

// ensureBucket — ленивая проверка/создание бакета (storage.service.ts → ensureBucket).
func (s *Service) ensureBucket(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client == nil {
		return common.StorageUnavailable("Файловое хранилище не настроено")
	}
	if s.initTried {
		return nil
	}

	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err == nil && !exists {
		err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
	}
	if err != nil {
		log.Printf("[Storage] MinIO bucket недоступен: %s", err.Error())
		return common.StorageUnavailable(common.MsgStorageDown)
	}
	s.initTried = true
	return nil
}

// Upload кладёт объект kind/owner/filename и возвращает публичный URL;
// при сбое — ошибка 503. kind ∈ resumes | avatars | logos (docs/07 §4).
// BUG-REF: F1 — публичный URL строится конкатенацией MINIO_PUBLIC_ENDPOINT + путь
// к объекту (без presigned); скачивания/удаления файлов нет — «удаление» резюме/аватара
// лишь перезаписывает поле профиля, файл остаётся в бакете.
func (s *Service) Upload(ctx context.Context, kind, owner, filename string, buf []byte, contentType string) (string, error) {
	if err := s.ensureBucket(ctx); err != nil {
		return "", err
	}
	key := kind + "/" + owner + "/" + filename
	if _, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(buf), int64(len(buf)),
		minio.PutObjectOptions{ContentType: contentType}); err != nil {
		log.Printf("[Storage] Ошибка загрузки %s: %s", key, err.Error())
		return "", common.StorageUnavailable("Не удалось сохранить файл")
	}
	return s.publicBase + "/" + s.bucket + "/" + key, nil
}
