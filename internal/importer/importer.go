package importer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/centaur-vova/sap-segmentation/internal/config"
	"github.com/centaur-vova/sap-segmentation/internal/model"
)

const (
	// batchChannelSize - размер буфера канала для батчей.
	batchChannelSize = 5

	// errorChannelSize - размер буфера канала для ошибок.
	errorChannelSize = 1

	// maxRetries - количество повторных попыток при ошибке запроса.
	maxRetries = 3
)

// ERPItem - элемент ответа от ERP системы.
type ERPItem struct {
	AddressSapID string `json:"address_sap_id"`
	AdrSegment   string `json:"adr_segment"`
	SegmentID    int64  `json:"segment_id"`
}

// APIResponse - ответ от ERP системы.
type APIResponse struct {
	Items []ERPItem `json:"items"`
}

type segmentationUpserter interface {
	UpsertBatch(ctx context.Context, segments []model.Segmentation) error
}

// Importer - импортер данных из ERP системы.
type Importer struct {
	config *config.Config
	logger *slog.Logger
	model  segmentationUpserter
	client *http.Client
}

// NewImporter - создает новый экземпляр импортера.
func NewImporter(cfg *config.Config, logger *slog.Logger, model segmentationUpserter) *Importer {
	client := &http.Client{
		Timeout: time.Duration(cfg.ConnTimeout) * time.Second,
		Transport: &http.Transport{
			// Общий лимит простаивающих соединений в пуле
			MaxIdleConns: 100,

			// Сколько keep-alive соединений держать для одного хоста
			MaxIdleConnsPerHost: 20,

			// Время жизни неиспользуемого соединения
			IdleConnTimeout: 90 * time.Second,

			// Таймаут TLS/TCP рукопожатия
			TLSHandshakeTimeout: 10 * time.Second,

			// Таймаут ожидания заголовков ответа
			ResponseHeaderTimeout: 5 * time.Second,

			// Диалер с таймаутами
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}

	return &Importer{
		config: cfg,
		logger: logger,
		model:  model,
		client: client,
	}
}

// Run - запускает процесс импорта данных.
func (i *Importer) Run(ctx context.Context) error {
	i.logger.Info("Starting SAP segmentation import process")

	batchChan := make(chan []model.Segmentation, batchChannelSize)
	errChan := make(chan error, errorChannelSize)
	workerDone := make(chan struct{})

	// Воркер для вставки в БД
	go func() {
		defer close(workerDone)
		for segments := range batchChan {
			if err := i.model.UpsertBatch(ctx, segments); err != nil {
				select {
				case errChan <- err:
				default:
				}
				return
			}
			i.logger.Info("Batch successfully upserted into DB", "count", len(segments))
		}
	}()

	offset := 1
	batchSize := i.config.ImportBatchSize
	interval := time.Duration(i.config.ConnInterval) * time.Millisecond
	totalImported := 0

	for {
		select {
		case <-ctx.Done():
			close(batchChan)
			<-workerDone
			return ctx.Err()
		case err := <-errChan:
			close(batchChan)
			<-workerDone
			return fmt.Errorf("database worker error: %w", err)
		default:
		}

		url := fmt.Sprintf("%s?p_limit=%d&p_offset=%d", i.config.ConnURI, batchSize, offset)
		i.logger.Info("Requesting ERP data", "endpoint", url, "offset", offset, "limit", batchSize)

		fetchStart := time.Now()
		data, err := i.fetchWithRetry(ctx, url, maxRetries)
		fetchDuration := time.Since(fetchStart)

		if err != nil {
			i.logger.Error("Failed to fetch data from API",
				"error", err,
				"url", url,
				"duration", fetchDuration.String(),
			)
			close(batchChan)
			<-workerDone
			return fmt.Errorf("failed to fetch data at offset %d: %w", offset, err)
		}

		i.logger.Debug("API request completed",
			"duration", fetchDuration.String(),
			"items_received", len(data.Items),
		)

		if len(data.Items) == 0 {
			i.logger.Info("Received empty response from ERP. Import finished.")
			break
		}

		segments := i.convertToModel(data.Items)

		select {
		case <-ctx.Done():
			close(batchChan)
			<-workerDone
			return ctx.Err()
		case batchChan <- segments:
		}

		totalImported += len(segments)

		i.logger.Info("Batch sent to worker",
			"offset", offset,
			"batch_size", len(segments),
			"total_imported", totalImported,
		)

		offset += batchSize

		i.logger.Debug("Sleeping between batches", "duration", interval.String())

		select {
		case <-ctx.Done():
			close(batchChan)
			<-workerDone
			return ctx.Err()
		case <-time.After(interval):
		}
	}

	close(batchChan)
	<-workerDone

	select {
	case err := <-errChan:
		return err
	default:
	}

	i.logger.Info("Import completed successfully", "total_imported", totalImported)
	return nil
}

func (i *Importer) convertToModel(items []ERPItem) []model.Segmentation {
	segments := make([]model.Segmentation, 0, len(items))
	for _, item := range items {
		segments = append(segments, model.Segmentation{
			AddressSapID: item.AddressSapID,
			AdrSegment:   item.AdrSegment,
			SegmentID:    item.SegmentID,
		})
	}
	return segments
}

func (i *Importer) fetchData(ctx context.Context, url string) (*APIResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", i.config.ConnUserAgent)

	auth := base64.StdEncoding.EncodeToString([]byte(i.config.ConnAuthLoginPwd))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))

	resp, err := i.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		limitReader := io.LimitReader(resp.Body, 4096)
		if _, err := io.Copy(io.Discard, limitReader); err != nil {
			i.logger.Debug("Failed to drain response body", "error", err)
		}
		if err := resp.Body.Close(); err != nil {
			i.logger.Error("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	return &apiResp, nil
}
