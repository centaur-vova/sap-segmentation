-- setup/install.sql
-- Создание таблицы segmentation
CREATE TABLE IF NOT EXISTS segmentation (
    id BIGSERIAL PRIMARY KEY,
    address_sap_id VARCHAR(255) NOT NULL,
    adr_segment VARCHAR(16) NOT NULL,
    segment_id BIGINT NOT NULL,

    -- Таймстемпы для отслеживания изменений
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Явное именованное ограничение уникальности
    CONSTRAINT uk_segmentation_address_sap_id UNIQUE (address_sap_id)
);

-- Индекс для ускорения поиска по address_sap_id
CREATE INDEX IF NOT EXISTS idx_segmentation_address_sap_id ON segmentation(address_sap_id);