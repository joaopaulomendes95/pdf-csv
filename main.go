package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"flag"
	"sync/atomic"
	"time"

	"github.com/dslipak/pdf"
)

// Config holds the configurable parameters for the program.
type Config struct {
	OutputFileName string
	NumWorkers     int
	TemplatePath   string
}

// RegexConfig holds the regex patterns loaded from the JSON file.
type RegexConfig struct {
	Fatura          string `json:"fatura"`
	ClienteMatricula string `json:"cliente_matricula"`
	DataInicio      string `json:"data_inicio"`
	Valor           string `json:"valor"`
	PrazoMeses      string `json:"prazo_meses"`
}

type Invoice struct {
	Fatura           string
	ClienteMatricula string
	DataInicio       string
	Valor            string
	PrazoMeses       string
}

// Global compiled regex patterns for valor cleaning
var (
	reValorCleanerDot   = regexp.MustCompile(`\.`)
	reValorCleanerComma = regexp.MustCompile(`,`)
)

func main() {
	startTime := time.Now()

	// Define command-line flags
	outputFileName := flag.String("output", "invoices.csv", "Output CSV file name")
	numWorkers := flag.Int("workers", 8, "Number of worker goroutines")
	templatePath := flag.String("template", "template.json", "Path to the JSON regex template file")

	flag.Parse() // Parse the command-line arguments

	cfg := Config{
		OutputFileName: *outputFileName,
		NumWorkers:     *numWorkers,
		TemplatePath:   *templatePath,
	}

	// Load regex configuration
	regexCfg, err := loadRegexConfig(cfg.TemplatePath)
	if err != nil {
		log.Fatalf("Failed to load regex configuration from %s: %v", cfg.TemplatePath, err)
	}

	log.Printf("Processing PDFs and writing to %s", cfg.OutputFileName)

	files, err := filepath.Glob("pdfs/*.pdf")
	if err != nil {
		log.Fatalf("Failed to find PDF files: %v", err)
	}
	log.Printf("Found %d PDF files to process.", len(files))

	jobs := make(chan string, len(files))
	results := make(chan Invoice, len(files))

	var wg sync.WaitGroup
	var totalProcessed, totalErrors uint64 // Atomic counters

	for i := 0; i < cfg.NumWorkers; i++ {
		wg.Add(1)
		go worker(&wg, jobs, results, &totalProcessed, &totalErrors, regexCfg)
	}

	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	wg.Wait()
	close(results)

	// Collect all invoices from the results channel
	var invoices []Invoice
	for invoice := range results {
		invoices = append(invoices, invoice)
	}

	// Sort invoices by Fatura number
	sort.Slice(invoices, func(i, j int) bool {
		faturaI := invoices[i].Fatura
		faturaJ := invoices[j].Fatura

		partsI := strings.Split(faturaI, "/")
		partsJ := strings.Split(faturaJ, "/")

		if len(partsI) < 2 || len(partsJ) < 2 {
			return faturaI < faturaJ // Fallback to string comparison
		}

		numI, errI := strconv.Atoi(strings.TrimSpace(partsI[1]))
		numJ, errJ := strconv.Atoi(strings.TrimSpace(partsJ[1]))

		if errI != nil || errJ != nil {
			return faturaI < faturaJ // Fallback to string comparison
		}

		return numI < numJ
	})

	// Write sorted invoices to CSV
	if err := WriteInvoicesToCSV(cfg.OutputFileName, invoices); err != nil {
		log.Fatalf("Failed to write invoices to CSV: %v", err)
	}

	log.Printf("Processing complete. Processed %d PDFs with %d errors in %s", atomic.LoadUint64(&totalProcessed), atomic.LoadUint64(&totalErrors), time.Since(startTime))
}

func loadRegexConfig(filePath string) (RegexConfig, error) {
	var config RegexConfig
	data, err := os.ReadFile(filePath)
	if err != nil {
		return config, fmt.Errorf("reading regex config file: %w", err)
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, fmt.Errorf("unmarshalling regex config: %w", err)
	}
	return config, nil
}

func worker(wg *sync.WaitGroup, jobs <-chan string, results chan<- Invoice, totalProcessed, totalErrors *uint64, regexCfg RegexConfig) {
	defer wg.Done()
	for job := range jobs {
		content, err := readPdf(job)
		if err != nil {
			log.Printf("Error reading PDF %s: %v", job, err)
			atomic.AddUint64(totalErrors, 1)
			continue
		}
		invoice := parseInvoice(content, regexCfg)
		results <- invoice
		atomic.AddUint64(totalProcessed, 1)
	}
}

func parseInvoice(text string, regexCfg RegexConfig) Invoice {
	invoice := Invoice{}

	reFatura := regexp.MustCompile(regexCfg.Fatura)
	reCliente := regexp.MustCompile(regexCfg.ClienteMatricula)
	reMatricula := regexp.MustCompile(`matricula\s+([A-Z0-9]{2}-[A-Z0-9]{2}-[A-Z0-9]{2})`)
	reDataInicio := regexp.MustCompile(regexCfg.DataInicio)
	reValor := regexp.MustCompile(regexCfg.Valor)
	rePrazoMeses := regexp.MustCompile(regexCfg.PrazoMeses)

	match := reFatura.FindStringSubmatch(text)
	if len(match) > 1 {
		invoice.Fatura = match[1]
	}

	clienteMatch := reCliente.FindStringSubmatch(text)
	matriculaMatch := reMatricula.FindStringSubmatch(text)
	if len(clienteMatch) > 1 && len(matriculaMatch) > 1 {
		invoice.ClienteMatricula = fmt.Sprintf("%s/%s", clienteMatch[1], matriculaMatch[1])
	} else if len(clienteMatch) > 1 {
		invoice.ClienteMatricula = clienteMatch[1]
	} else if len(matriculaMatch) > 1 {
		invoice.ClienteMatricula = matriculaMatch[1]
	}

	match = reDataInicio.FindStringSubmatch(text)
	if len(match) > 3 {
		invoice.DataInicio = fmt.Sprintf("%s-%s-%s", match[3], match[2], match[1])
	}

	match = reValor.FindStringSubmatch(text)
	if len(match) > 1 {
		cleanedValor := match[1]
		cleanedValor = reValorCleanerDot.ReplaceAllString(cleanedValor, "")
		invoice.Valor = cleanedValor
	}

	match = rePrazoMeses.FindStringSubmatch(text)
	if len(match) > 1 {
		invoice.PrazoMeses = match[1]
	}

	return invoice
}

func readPdf(path string) (string, error) {
	r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	buf.ReadFrom(b)
	return buf.String(), nil
}

// WriteInvoicesToCSV writes a slice of Invoice structs to a CSV file.
func WriteInvoicesToCSV(filename string, invoices []Invoice) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file %s: %w", filename, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"Fatura", "Cliente/Matricula", "Data Inicio", "Valor", "Prazo Meses"}); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write invoice data
	for _, invoice := range invoices {
		record := []string{
			invoice.Fatura,
			invoice.ClienteMatricula,
			invoice.DataInicio,
			invoice.Valor,
			invoice.PrazoMeses,
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	log.Printf("Successfully wrote invoices to %s", filename)
	return nil
}
