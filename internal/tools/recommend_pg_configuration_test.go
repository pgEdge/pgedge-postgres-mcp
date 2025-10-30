package tools

import (
	"strings"
	"testing"
)

func TestCalculateSharedBuffers(t *testing.T) {
	tests := []struct {
		name       string
		ramGB      float64
		wantMin    float64
		wantMax    float64
		wantCapped bool
	}{
		{"Small RAM (2GB)", 2, 256, 512, false},
		{"Medium RAM (8GB)", 8, 1536, 2048, false},
		{"Large RAM (32GB)", 32, 8000, 8192, false},
		{"Very Large RAM (128GB)", 128, 21000, 22000, false},
		{"Huge RAM (256GB)", 256, 43600, 43800, false},   // 256GB / 6 = ~42.67GB
		{"Extremely Large RAM (512GB)", 512, 65535, 65536, true}, // Capped at 64GB
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSharedBuffers(tt.ramGB)
			if result < tt.wantMin || result > tt.wantMax {
				t.Errorf("calculateSharedBuffers(%v) = %v, want between %v and %v",
					tt.ramGB, result, tt.wantMin, tt.wantMax)
			}
			if tt.wantCapped && result > 65536 {
				t.Errorf("calculateSharedBuffers(%v) = %v, should be capped at 64GB (65536MB)",
					tt.ramGB, result)
			}
		})
	}
}

func TestCalculateWorkMem(t *testing.T) {
	tests := []struct {
		name             string
		ramGB            float64
		sharedBuffersMB  float64
		cpuCores         int
		workloadType     string
		expectGreaterThan float64
	}{
		{"OLTP Small", 16, 4096, 4, "OLTP", 4},
		{"OLAP Medium", 32, 8192, 8, "OLAP", 20},
		{"Mixed Large", 64, 16384, 16, "Mixed", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateWorkMem(tt.ramGB, tt.sharedBuffersMB, tt.cpuCores, tt.workloadType)
			if float64(result) < tt.expectGreaterThan {
				t.Errorf("calculateWorkMem() = %v, want > %v", result, tt.expectGreaterThan)
			}
			if result > 512 {
				t.Errorf("calculateWorkMem() = %v, should be capped at 512MB", result)
			}
			if result < 4 {
				t.Errorf("calculateWorkMem() = %v, should be minimum 4MB", result)
			}
		})
	}
}

func TestCalculateMaintenanceWorkMem(t *testing.T) {
	tests := []struct {
		name               string
		ramGB              float64
		sharedBuffersMB    float64
		autovacuumWorkers  int
		expectLessThanOrEq int
	}{
		{"Small RAM", 8, 2048, 5, 1024},
		{"Medium RAM", 32, 8192, 5, 1024},
		{"Large RAM", 128, 16384, 5, 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateMaintenanceWorkMem(tt.ramGB, tt.sharedBuffersMB, tt.autovacuumWorkers)
			if result > tt.expectLessThanOrEq {
				t.Errorf("calculateMaintenanceWorkMem() = %v, should be <= %v", result, tt.expectLessThanOrEq)
			}
			if result < 64 {
				t.Errorf("calculateMaintenanceWorkMem() = %v, should be >= 64MB", result)
			}
		})
	}
}

func TestCalculateEffectiveCacheSize(t *testing.T) {
	tests := []struct {
		name            string
		ramGB           float64
		sharedBuffersGB float64
		expectMin       float64
	}{
		{"Small System", 8, 2, 5},
		{"Medium System", 32, 8, 20},
		{"Large System", 128, 16, 72},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateEffectiveCacheSize(tt.ramGB, tt.sharedBuffersGB)
			if result < tt.expectMin {
				t.Errorf("calculateEffectiveCacheSize() = %v, want >= %v", result, tt.expectMin)
			}
			if result > tt.ramGB {
				t.Errorf("calculateEffectiveCacheSize() = %v, should not exceed total RAM %v", result, tt.ramGB)
			}
		})
	}
}

func TestCalculateMaxWALSize(t *testing.T) {
	tests := []struct {
		name         string
		diskSpaceGB  float64
		workloadType string
		expect       string
	}{
		{"No disk info OLTP", 0, "OLTP", "4GB"},
		{"No disk info OLAP", 0, "OLAP", "16GB"},
		{"100GB disk OLTP", 100, "OLTP", "30GB"},
		{"500GB disk OLAP", 500, "OLAP", "200GB"}, // Capped at 200GB
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateMaxWALSize(tt.diskSpaceGB, tt.workloadType)
			if result != tt.expect {
				t.Errorf("calculateMaxWALSize() = %v, want %v", result, tt.expect)
			}
		})
	}
}

func TestGenerateConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		ramGB        float64
		cpuCores     int
		storageType  string
		workloadType string
		vmEnv        bool
		separateWAL  bool
		diskSpaceGB  float64
	}{
		{"Small OLTP HDD", 8, 4, "HDD", "OLTP", false, false, 0},
		{"Medium Mixed SSD", 32, 8, "SSD", "Mixed", false, false, 0},
		{"Large OLAP NVMe VM", 128, 32, "NVMe", "OLAP", true, true, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := generateConfiguration(tt.ramGB, tt.cpuCores, tt.storageType,
				tt.workloadType, tt.vmEnv, tt.separateWAL, tt.diskSpaceGB)

			if len(config) == 0 {
				t.Error("generateConfiguration returned empty recommendations")
			}

			// Check that key parameters are present
			foundParams := make(map[string]bool)
			for _, rec := range config {
				foundParams[rec.Parameter] = true
			}

			requiredParams := []string{
				"max_connections", "shared_buffers", "work_mem",
				"maintenance_work_mem", "effective_cache_size",
				"wal_buffers", "checkpoint_timeout", "max_wal_size",
			}

			for _, param := range requiredParams {
				if !foundParams[param] {
					t.Errorf("Missing required parameter: %s", param)
				}
			}

			// Check VM-specific parameters
			if tt.vmEnv {
				if !foundParams["wal_recycle"] {
					t.Error("VM environment should include wal_recycle parameter")
				}
				if !foundParams["wal_init_zero"] {
					t.Error("VM environment should include wal_init_zero parameter")
				}
			}

			// Check storage-specific parameters
			for _, rec := range config {
				if rec.Parameter == "random_page_cost" {
					if tt.storageType == "SSD" || tt.storageType == "NVMe" {
						if rec.Value != "1.1" {
							t.Errorf("random_page_cost for %s should be 1.1, got %s",
								tt.storageType, rec.Value)
						}
					} else if tt.storageType == "HDD" {
						if rec.Value != "4.0" {
							t.Errorf("random_page_cost for HDD should be 4.0, got %s", rec.Value)
						}
					}
				}
				if rec.Parameter == "effective_io_concurrency" {
					if tt.storageType == "HDD" {
						if rec.Value != "2" {
							t.Errorf("effective_io_concurrency for HDD should be 2, got %s", rec.Value)
						}
					} else {
						if rec.Value != "200" {
							t.Errorf("effective_io_concurrency for SSD/NVMe should be 200, got %s", rec.Value)
						}
					}
				}
			}
		})
	}
}

func TestRecommendPGConfigurationTool(t *testing.T) {
	tool := RecommendPGConfigurationTool()

	// Test tool definition
	if tool.Definition.Name != "recommend_pg_configuration" {
		t.Errorf("Tool name = %v, want recommend_pg_configuration", tool.Definition.Name)
	}

	if tool.Definition.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Verify tool description emphasizes NEW installations only
	desc := tool.Definition.Description
	if !strings.Contains(desc, "STARTING POINT") {
		t.Error("Tool description should emphasize this is a starting point")
	}
	if !strings.Contains(desc, "NEW installations only") {
		t.Error("Tool description should state this is for NEW installations only")
	}
	if !strings.Contains(desc, "NOT for fine-tuning") {
		t.Error("Tool description should warn against using for fine-tuning existing systems")
	}

	// Test required parameters
	required := tool.Definition.InputSchema.Required
	expectedRequired := []string{"total_ram_gb", "cpu_cores", "storage_type", "workload_type"}
	if len(required) != len(expectedRequired) {
		t.Errorf("Required parameters count = %v, want %v", len(required), len(expectedRequired))
	}

	// Test valid input
	t.Run("Valid Input", func(t *testing.T) {
		input := map[string]interface{}{
			"total_ram_gb":   32.0,
			"cpu_cores":      8.0,
			"storage_type":   "SSD",
			"workload_type":  "Mixed",
			"vm_environment": false,
		}

		result, err := tool.Handler(input)
		if err != nil {
			t.Errorf("Handler failed with valid input: %v", err)
		}

		if len(result.Content) == 0 {
			t.Error("Handler returned empty content")
		}

		text := result.Content[0].Text

		// Check that result contains key configuration parameters
		if !strings.Contains(text, "shared_buffers") {
			t.Error("Result should contain shared_buffers")
		}
		if !strings.Contains(text, "max_connections") {
			t.Error("Result should contain max_connections")
		}
		if !strings.Contains(text, "work_mem") {
			t.Error("Result should contain work_mem")
		}
	})

	// Test invalid inputs
	t.Run("Invalid RAM", func(t *testing.T) {
		input := map[string]interface{}{
			"total_ram_gb":  -1.0,
			"cpu_cores":     8.0,
			"storage_type":  "SSD",
			"workload_type": "OLTP",
		}

		result, err := tool.Handler(input)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("Handler should return error response with negative RAM")
		}
	})

	t.Run("Invalid CPU Cores", func(t *testing.T) {
		input := map[string]interface{}{
			"total_ram_gb":  32.0,
			"cpu_cores":     0.0,
			"storage_type":  "SSD",
			"workload_type": "OLTP",
		}

		result, err := tool.Handler(input)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("Handler should return error response with zero CPU cores")
		}
	})

	t.Run("Missing Parameters", func(t *testing.T) {
		input := map[string]interface{}{
			"total_ram_gb": 32.0,
			// Missing required parameters
		}

		result, err := tool.Handler(input)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("Handler should return error response with missing parameters")
		}
	})
}

func TestFormatConfigurationOutput(t *testing.T) {
	recommendations := []configRecommendation{
		{
			Parameter:   "shared_buffers",
			Value:       "8GB",
			Explanation: "Test explanation",
			Section:     "Memory",
		},
		{
			Parameter:   "max_connections",
			Value:       "100",
			Explanation: "Test explanation",
			Section:     "Connection Management",
		},
	}

	output := formatConfigurationOutput(recommendations)

	if output == "" {
		t.Error("formatConfigurationOutput returned empty string")
	}

	// Check for key sections
	if !strings.Contains(output, "PostgreSQL Configuration Recommendations for NEW Installations") {
		t.Error("Output should contain title emphasizing NEW installations")
	}

	// Check for critical warnings
	if !strings.Contains(output, "STARTING POINTS for NEW PostgreSQL deployments") {
		t.Error("Output should contain warning about starting points")
	}
	if !strings.Contains(output, "DO NOT apply to existing production systems") {
		t.Error("Output should contain warning about existing systems")
	}
	if !strings.Contains(output, "BASELINE settings for NEW installations ONLY") {
		t.Error("Output should contain critical warning about baseline settings")
	}

	if !strings.Contains(output, "Connection Management") {
		t.Error("Output should contain Connection Management section")
	}
	if !strings.Contains(output, "Memory") {
		t.Error("Output should contain Memory section")
	}
	if !strings.Contains(output, "shared_buffers") {
		t.Error("Output should contain shared_buffers parameter")
	}
	if !strings.Contains(output, "max_connections") {
		t.Error("Output should contain max_connections parameter")
	}
	if !strings.Contains(output, "Additional Recommendations") {
		t.Error("Output should contain Additional Recommendations section")
	}
	if !strings.Contains(output, "Operating System Tuning") {
		t.Error("Output should contain OS tuning recommendations")
	}
}

func TestWorkloadTypeAdjustments(t *testing.T) {
	ramGB := 32.0
	sharedBuffersMB := 8192.0
	cpuCores := 8

	oltpWorkMem := calculateWorkMem(ramGB, sharedBuffersMB, cpuCores, "OLTP")
	olapWorkMem := calculateWorkMem(ramGB, sharedBuffersMB, cpuCores, "OLAP")
	mixedWorkMem := calculateWorkMem(ramGB, sharedBuffersMB, cpuCores, "Mixed")

	// OLAP should get more work_mem than OLTP
	if olapWorkMem <= oltpWorkMem {
		t.Errorf("OLAP work_mem (%d) should be greater than OLTP work_mem (%d)",
			olapWorkMem, oltpWorkMem)
	}

	// Mixed should be between OLTP and OLAP
	if mixedWorkMem < oltpWorkMem || mixedWorkMem > olapWorkMem {
		t.Errorf("Mixed work_mem (%d) should be between OLTP (%d) and OLAP (%d)",
			mixedWorkMem, oltpWorkMem, olapWorkMem)
	}
}
