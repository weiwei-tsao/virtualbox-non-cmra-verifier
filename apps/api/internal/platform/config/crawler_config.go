package config

import (
	"context"
	"log"
	"os"
	"reflect"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/goccy/go-yaml"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// ConfigLoader implements 5-level configuration cascade:
// Level 1: Code defaults
// Level 2: YAML file (config.yaml)
// Level 3: Environment variables
// Level 4: Database (Firestore system_config/crawler_config)
// Level 5: Per-job parameters (runtime overrides)
type ConfigLoader struct {
	firestoreClient *firestore.Client
}

// NewConfigLoader creates a new configuration loader.
func NewConfigLoader(client *firestore.Client) *ConfigLoader {
	return &ConfigLoader{
		firestoreClient: client,
	}
}

// Load cascades through all config sources and returns the final merged configuration.
func (l *ConfigLoader) Load(ctx context.Context) (model.CrawlerConfig, error) {
	// Level 1: Start with code defaults
	cfg := model.DefaultCrawlerConfig()

	// Level 2: Overlay YAML config
	yamlCfg, err := l.loadYAML()
	if err == nil {
		cfg = mergeConfigs(cfg, yamlCfg)
	} else if !os.IsNotExist(err) {
		// Log YAML errors but don't fail (use defaults)
		log.Printf("Warning: Failed to load config.yaml: %v", err)
	}

	// Level 3: Overlay environment variables
	envCfg := l.loadEnv()
	cfg = mergeConfigs(cfg, envCfg)

	// Level 4: Overlay database config (if Firestore is available)
	if l.firestoreClient != nil {
		dbCfg, err := l.loadDB(ctx)
		if err == nil {
			cfg = mergeConfigs(cfg, dbCfg)
		} else {
			// Log DB errors but don't fail (DB config is optional)
			log.Printf("Warning: Failed to load DB config: %v", err)
		}
	}

	// Level 5 is applied at runtime via MergeJobOverrides()

	return cfg, nil
}

// loadYAML reads configuration from config.yaml file.
func (l *ConfigLoader) loadYAML() (model.CrawlerConfig, error) {
	var cfg model.CrawlerConfig

	data, err := os.ReadFile("config.yaml")
	if err != nil {
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// loadEnv reads configuration from environment variables.
func (l *ConfigLoader) loadEnv() model.CrawlerConfig {
	var cfg model.CrawlerConfig

	// Scraping settings
	if val := os.Getenv("CRAWLER_WORKER_COUNT"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			cfg.WorkerCount = n
		}
	}
	if val := os.Getenv("CRAWLER_SCRAPE_BATCH_SIZE"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			cfg.ScrapeBatchSize = n
		}
	}
	if val := os.Getenv("CRAWLER_SCRAPE_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.ScrapeTimeout = d
		}
	}

	// Validation settings
	if val := os.Getenv("CRAWLER_VALIDATION_BATCH_SIZE"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			cfg.ValidationBatchSize = n
		}
	}
	if val := os.Getenv("CRAWLER_VALIDATION_WORKER_COUNT"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			cfg.ValidationWorkerCount = n
		}
	}
	if val := os.Getenv("CRAWLER_MAX_RETRY_ATTEMPTS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			cfg.MaxRetryAttempts = n
		}
	}
	if val := os.Getenv("CRAWLER_RETRY_BACKOFF_BASE"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.RetryBackoffBase = d
		}
	}
	if val := os.Getenv("CRAWLER_RETRY_BACKOFF_MULTIPLIER"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.RetryBackoffMultiplier = f
		}
	}
	if val := os.Getenv("CRAWLER_RETRY_BACKOFF_JITTER"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.RetryBackoffJitter = f
		}
	}

	// Re-validation settings
	if val := os.Getenv("CRAWLER_REVALIDATION_INTERVAL"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.RevalidationInterval = d
		}
	}
	if val := os.Getenv("CRAWLER_REVALIDATION_THRESHOLD"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			cfg.RevalidationThreshold = d
		}
	}

	// Quota management
	if val := os.Getenv("CRAWLER_DAILY_VALIDATION_BUDGET"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			cfg.DailyValidationBudget = n
		}
	}
	if val := os.Getenv("CRAWLER_HIGH_PRIORITY_QUOTA"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			cfg.HighPriorityQuota = n
		}
	}
	if val := os.Getenv("CRAWLER_MEDIUM_PRIORITY_QUOTA"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			cfg.MediumPriorityQuota = n
		}
	}

	return cfg
}

// loadDB reads configuration from Firestore system_config/crawler_config document.
func (l *ConfigLoader) loadDB(ctx context.Context) (model.CrawlerConfig, error) {
	var cfg model.CrawlerConfig

	doc, err := l.firestoreClient.Collection("system_config").Doc("crawler_config").Get(ctx)
	if err != nil {
		// Document might not exist yet, that's okay
		if statusErr, ok := err.(interface{ GRPCStatus() interface{ Code() int } }); ok {
			if statusErr.GRPCStatus().Code() == 5 { // NotFound
				return cfg, nil
			}
		}
		return cfg, err
	}

	if err := doc.DataTo(&cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// SaveToDB saves the current configuration to Firestore for persistent storage.
func (l *ConfigLoader) SaveToDB(ctx context.Context, cfg model.CrawlerConfig) error {
	if l.firestoreClient == nil {
		return nil // Firestore not available, skip
	}

	_, err := l.firestoreClient.Collection("system_config").Doc("crawler_config").Set(ctx, cfg)
	return err
}

// MergeJobOverrides applies per-job configuration overrides to a base configuration.
// This is Level 5 of the cascade, applied at runtime when starting a job.
func (l *ConfigLoader) MergeJobOverrides(base model.CrawlerConfig, overrides map[string]interface{}) model.CrawlerConfig {
	if len(overrides) == 0 {
		return base
	}

	// Use reflection to apply overrides
	result := base
	v := reflect.ValueOf(&result).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			continue
		}

		// Check if override exists for this field
		if overrideVal, ok := overrides[jsonTag]; ok && overrideVal != nil {
			fieldVal := v.Field(i)
			if fieldVal.CanSet() {
				setFieldValue(fieldVal, overrideVal)
			}
		}
	}

	return result
}

// setFieldValue sets a reflect.Value from an interface{} with type conversion.
func setFieldValue(field reflect.Value, value interface{}) {
	switch field.Kind() {
	case reflect.Int:
		if v, ok := value.(float64); ok {
			field.SetInt(int64(v))
		} else if v, ok := value.(int); ok {
			field.SetInt(int64(v))
		}
	case reflect.Float64:
		if v, ok := value.(float64); ok {
			field.SetFloat(v)
		}
	case reflect.String:
		if v, ok := value.(string); ok {
			field.SetString(v)
		}
	case reflect.Int64: // time.Duration
		if v, ok := value.(string); ok {
			if d, err := time.ParseDuration(v); err == nil {
				field.SetInt(int64(d))
			}
		}
	}
}

// mergeConfigs merges two configurations, with 'override' taking precedence over 'base'.
// Only non-zero values from override are applied.
func mergeConfigs(base, override model.CrawlerConfig) model.CrawlerConfig {
	result := base

	// Use reflection to merge non-zero fields
	vOverride := reflect.ValueOf(override)
	vResult := reflect.ValueOf(&result).Elem()

	for i := 0; i < vOverride.NumField(); i++ {
		overrideField := vOverride.Field(i)
		resultField := vResult.Field(i)

		// Only apply non-zero values
		if !isZeroValue(overrideField) && resultField.CanSet() {
			resultField.Set(overrideField)
		}
	}

	return result
}

// isZeroValue checks if a reflect.Value is the zero value for its type.
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Int, reflect.Int64:
		return v.Int() == 0
	case reflect.Float64:
		return v.Float() == 0
	case reflect.String:
		return v.String() == ""
	default:
		return false
	}
}

// GetAllConfigSources returns a debug view of configuration from all sources.
// Useful for troubleshooting which values come from which source.
func (l *ConfigLoader) GetAllConfigSources(ctx context.Context) map[string]model.CrawlerConfig {
	sources := make(map[string]model.CrawlerConfig)

	sources["1_defaults"] = model.DefaultCrawlerConfig()

	if yamlCfg, err := l.loadYAML(); err == nil {
		sources["2_yaml"] = yamlCfg
	}

	sources["3_env"] = l.loadEnv()

	if l.firestoreClient != nil {
		if dbCfg, err := l.loadDB(ctx); err == nil {
			sources["4_db"] = dbCfg
		}
	}

	return sources
}
