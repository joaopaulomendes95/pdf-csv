# PDF to CSV Converter

This Go application efficiently extracts invoice data from PDF files and exports it into a CSV format. It leverages Go's concurrency features to process multiple PDF files in parallel using a worker pool, and streams the results directly to the CSV output to maintain low memory usage. The parsing logic for extracting data from PDFs is externalized into a JSON configuration file, allowing for flexible adaptation to different invoice layouts without recompiling the code.

## Features

*   **Concurrent Processing**: Utilizes a worker pool with goroutines and channels for fast, parallel PDF processing.
*   **Configurable Parsing Logic**: Regular expressions for data extraction are defined in an external `template.json` file, enabling easy adaptation to new invoice formats.
*   **Memory Efficient Streaming**: Processes and writes invoice data directly to the CSV file as it becomes available, preventing high memory consumption for large datasets.
*   **Atomic Statistics**: Uses `sync/atomic` for efficient and thread-safe counting of processed files and errors.
*   **Command-Line Configuration**: Supports command-line flags for specifying the output file name, number of workers, and the path to the regex template file.
*   **Clear Separation of Concerns**: Well-structured code with distinct functions for reading PDFs, parsing content, and writing CSV.

## Prerequisites

Before running the application, ensure you have:

*   **Go (1.16 or higher)** installed on your system.
*   The `github.com/dslipak/pdf` library.

## Installation

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/joaopaulomendes95/pdf-to-csv.git
    cd pdf-csv
    ```

2.  **Install dependencies:**
    ```bash
    go mod tidy
    ```

3.  **Build the application:**
    ```bash
    go build -o pdf-to-csv
    ```

## Usage

1.  **Prepare your PDF files**: Place all PDF invoice files you wish to process into a directory named `pdfs` in the root of the project.

2.  **Configure the regex patterns**: Create a `template.json` file in the project root with the regular expressions tailored to your PDF invoice layout. An example is provided in the [Configuration](#configuration) section below.

3.  **Run the application**:
    ```bash
    ./pdf-to-csv [options]
    ```

    **Command-line Options:**
    *   `-output <filename>`: Specify the output CSV file name (default: `invoices.csv`).
    *   `-workers <number>`: Set the number of worker goroutines for parallel processing (default: `8`).
    *   `-template <path>`: Provide the path to the JSON regex template file (default: `template.json`).

    **Example:**
    ```bash
    ./pdf-to-csv -output my_invoices.csv -workers 16 -template config/invoice_template.json
    ```

## Configuration

The `template.json` file defines the regular expressions used to extract specific fields from the PDF text. Each key in the JSON corresponds to an invoice field, and its value is the regex pattern.

**Example `template.json` content:**

```json
{
  "fatura": "Fatura\\s*(?:/\\s*Invoice)?\\s*(FT\\s+\\d{4}/\\d+)",
  "cliente_matricula": "Cliente\\s*(?:/\\s*Customer)?\\s*(.*?)(?:Data de Vencimento|Data)",
  "data_inicio": "Data(?:\\s*de\\s*Vencimento)?\\s*(?:/\\s*Due\\s*date)?(\\d{4})-(\\d{2})-(\\d{2})",
  "valor": "Total\\s*([\\d,.]+)",
  "prazo_meses": "(?:[Gg]arantia(?:\\s+\\w+)*?\\s+)?(\\d{1,2})\\s+[Mm]eses"
}
```

**Important Note on Escaping**: When defining regex patterns in JSON, backslashes (`\`) must be escaped. For example, `\s` (whitespace) in a regular expression becomes `\\s` in the JSON string. The example above uses `\\s` because the `write_file` tool used during development required double escaping to correctly write `\s` to the file. If you are manually creating or editing `template.json`, ensure `\s` is written as `\\s`, `\d` as `\\d`, etc.

## Project Structure

```
pdf-to-csv/
├───.gitignore
├───go.mod
├───go.sum
├───main.go
├───README.md
├───template.json
└───pdfs/
    ├───invoice1.pdf
    ├───invoice2.pdf
    └───...
```

## Output

The application will generate a CSV file (default: `invoices.csv`) in the project root directory. This file will contain a header row followed by the extracted data for each processed invoice.

**Example `invoices.csv` output:**

```csv
Fatura,Cliente/Matricula,Data Inicio,Valor,Prazo Meses
FT 2023/12345,Test Client/AB-CD-EF,10/26/2023,1234.56,12
FT 2023/1001,Another Customer/XY-78-ZW,11/08/2023,387.45,18
...
```
