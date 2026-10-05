package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"arunika_backend/models"
	"arunika_backend/storage"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AssetService accepts admin uploads and resolves asset ids to public URLs.
type AssetService struct {
	db    *gorm.DB
	store storage.Store
	audit *AuditService
}

func NewAssetService(db *gorm.DB, store storage.Store, audit *AuditService) *AssetService {
	return &AssetService{db: db, store: store, audit: audit}
}

// AssetView is an asset with its public URL.
type AssetView struct {
	models.ContentAsset
	URL string `json:"url"`
}

func (s *AssetService) view(a models.ContentAsset) AssetView {
	return AssetView{ContentAsset: a, URL: s.store.URL(a.StorageKey)}
}

// Upload validates data as kind and stores it under huruf/<sha256>.<ext>.
// Bytes already uploaded return the existing asset without writing again.
func (s *AssetService) Upload(ctx context.Context, adminID uuid.UUID, kind, originalName string, data []byte) (*AssetView, bool, error) {
	info, err := inspectAsset(kind, data)
	if err != nil {
		return nil, false, err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	var existing models.ContentAsset
	err = s.db.Where("sha256 = ?", hash).First(&existing).Error
	if err == nil {
		v := s.view(existing)
		return &v, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	key := fmt.Sprintf("huruf/%s.%s", hash, info.ext)
	// Store first: an object without a row is harmless (re-uploading reuses
	// the key), a row pointing at a missing object is not.
	if err := s.store.Put(ctx, key, info.mime, data); err != nil {
		return nil, false, fmt.Errorf("store asset: %w", err)
	}
	asset := models.ContentAsset{
		Kind:         kind,
		StorageKey:   key,
		Sha256:       hash,
		Bytes:        len(data),
		Mime:         info.mime,
		Width:        info.width,
		Height:       info.height,
		DurationMs:   info.durationMs,
		OriginalName: truncate(originalName, 200),
		CreatedBy:    adminID,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&asset).Error; err != nil {
			return err
		}
		return s.audit.Record(tx, adminID, AuditUpload, "asset", asset.ID.String(), nil,
			map[string]interface{}{"kind": kind, "name": asset.OriginalName, "sha256": hash, "bytes": asset.Bytes})
	})
	if err != nil {
		// A concurrent upload of the same bytes won the unique index.
		if s.db.Where("sha256 = ?", hash).First(&existing).Error == nil {
			v := s.view(existing)
			return &v, false, nil
		}
		return nil, false, err
	}
	v := s.view(asset)
	return &v, true, nil
}

// Find loads assets by id; missing ids are simply absent from the map.
func (s *AssetService) Find(tx *gorm.DB, ids []uuid.UUID) (map[uuid.UUID]AssetView, error) {
	out := map[uuid.UUID]AssetView{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []models.ContentAsset
	if err := tx.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = s.view(r)
	}
	return out, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
