package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)


type Metadata struct {
	NUsers        int `json:"n_users"`
	NItems        int `json:"n_items"`
	NInteractions int `json:"n_interactions"`
}

// Interaction representa una fila del CSV
type Interaction struct {
	UserIndex int
	ItemIndex int
	Rating    float64
}

// Constantes matemáticas del Descenso de Gradiente (SGD)
const (
	LatentFeatures = 10     // Dimensión de los factores latentes (K)
	LearningRate   = 0.01   // Alpha
	Regularization = 0.05   // Lambda (para evitar overfitting)
)

// Modelo almacena las matrices P (Usuarios) y Q (Ítems)
type Modelo struct {
	P           [][]float64
	Q           [][]float64
	ItemMutexes []sync.Mutex // Arreglo de candados para Fine-Grained Locking
}


func main() {
	metaFile, err := os.Open("matriz_meta.json")
	if err != nil {
		log.Fatalf("Error abriendo metadata: %v", err)
	}
	defer metaFile.Close()

	var meta Metadata
	json.NewDecoder(metaFile).Decode(&meta)
	fmt.Printf("Metadata: %d Usuarios, %d Ítems, %d Interacciones\n", meta.NUsers, meta.NItems, meta.NInteractions)

	interacciones := cargarCSV("instacart_matriz_limpia.csv", meta.NInteractions)
	numWorkers := 12
	repeticiones := 10

	var tiemposSecuencial []float64
	var tiemposConcurrente []float64
	var speedups []float64

	fmt.Println("\n--- Iniciando Bateria de Pruebas (10 Ejecuciones) ---")
	fmt.Printf("%-10s | %-20s | %-20s | %-10s\n", "Ejecución", "T. Secuencial (ms)", "T. Concurrente (ms)", "Speedup")
	fmt.Println("-----------------------------------------------------------------------")

	for iter := 1; iter <= repeticiones; iter++ {
		// Ejecucion Secuencial
		modeloSec := inicializarModelo(meta.NUsers, meta.NItems)
		inicioSec := time.Now()
		EntrenarSecuencial(modeloSec, interacciones)
		tSec := float64(time.Since(inicioSec).Milliseconds())
		tiemposSecuencial = append(tiemposSecuencial, tSec)

		// Ejecucion Concurrente
		modeloConc := inicializarModelo(meta.NUsers, meta.NItems)
		inicioConc := time.Now()
		EntrenarConcurrente(modeloConc, interacciones, numWorkers)
		tConc := float64(time.Since(inicioConc).Milliseconds())
		tiemposConcurrente = append(tiemposConcurrente, tConc)

		// Speedup de esta iteración
		sp := tSec / tConc
		speedups = append(speedups, sp)

		fmt.Printf("%-10d | %-20.2f | %-20.2f | %-10.2fx\n", iter, tSec, tConc, sp)
	}

	// CÁLCULO DE MEDIA RECORTADA (Descartar el peor y el mejor caso)
	sort.Float64s(tiemposSecuencial)
	sort.Float64s(tiemposConcurrente)
	sort.Float64s(speedups)

	var sumaSec, sumaConc, sumaSp float64
	// Sumamos desde el índice 1 hasta n-2 (excluyendo los extremos)
	for i := 1; i < repeticiones-1; i++ {
		sumaSec += tiemposSecuencial[i]
		sumaConc += tiemposConcurrente[i]
		sumaSp += speedups[i]
	}

	nValores := float64(repeticiones - 2)
	mediaRecSec := sumaSec / nValores
	mediaRecConc := sumaConc / nValores
	mediaRecSp := sumaSp / nValores

	fmt.Println("\n=======================================================================")
	fmt.Println("                       ESTADISTICA FINAL (MEDIA RECORTADA)             ")
	fmt.Println("=======================================================================")
	fmt.Printf("Media Recortada Secuencial : %.2f ms\n", mediaRecSec)
	fmt.Printf("Media Recortada Concurrente: %.2f ms\n", mediaRecConc)
	fmt.Printf("Speedup Global Estabilizado: %.2fx\n", mediaRecSp)
	fmt.Println("=======================================================================")
}


// EntrenarSecuencial procesa la matriz en un solo hilo (One-by-One)
func EntrenarSecuencial(m *Modelo, interacciones []Interaction) {
	for _, obs := range interacciones {
		actualizarFactores(m, obs)
	}
}

// EntrenarConcurrente optimizado con procesamiento por Lotes (Chunking)
func EntrenarConcurrente(m *Modelo, interacciones []Interaction, numWorkers int) {
	var wg sync.WaitGroup

	chunkSize := 50000 // Tamaño del lote para amortizar la sobrecarga 
	numChunks := (len(interacciones) + chunkSize - 1) / chunkSize

	// transporte por paquetes de interacciones
	canalLotes := make(chan []Interaction, numChunks)

	// Lanzamos los Workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			// El worker toma un paquete entero y lo procesa internamente
			for lote := range canalLotes {
				for _, obs := range lote {
					actualizarFactoresConcurrente(m, obs)
				}
			}
		}(i)
	}

	// Partimos el arreglo gigante en pequeños lotes y enviamos al canal
	for i := 0; i < len(interacciones); i += chunkSize {
		fin := i + chunkSize
		if fin > len(interacciones) {
			fin = len(interacciones)
		}
		canalLotes <- interacciones[i:fin] // Enviamos la referencia al lote
	}
	close(canalLotes)

	// Esperar a que todos los Workers terminen
	wg.Wait()
}


// Logica Matematica Secuencial
func actualizarFactores(m *Modelo, obs Interaction) {
	u := obs.UserIndex
	i := obs.ItemIndex

	// Predecir y calcular error
	prediccion := 0.0
	for k := 0; k < LatentFeatures; k++ {
		prediccion += m.P[u][k] * m.Q[i][k]
	}
	errorPred := obs.Rating - prediccion

	// Actualizar factores latentes (Stochastic Gradient Descent)
	for k := 0; k < LatentFeatures; k++ {
		tempPu := m.P[u][k]
		m.P[u][k] += LearningRate * (errorPred*m.Q[i][k] - Regularization*m.P[u][k])
		m.Q[i][k] += LearningRate * (errorPred*tempPu - Regularization*m.Q[i][k])
	}
}

// Logica Matematica Concurrente (Con Fine-Grained Locking)
func actualizarFactoresConcurrente(m *Modelo, obs Interaction) {
	u := obs.UserIndex
	i := obs.ItemIndex

	// Prediccion local (lectura es segura)
	prediccion := 0.0
	for k := 0; k < LatentFeatures; k++ {
		prediccion += m.P[u][k] * m.Q[i][k] 
	}
	errorPred := obs.Rating - prediccion

	// Actualizacion asimetrica demostrada en el paper A-ASGD y simulada en Promela
	
	// 1. Usuarios: Lock-Free (baja probabilidad de colisión)
	for k := 0; k < LatentFeatures; k++ {
		m.P[u][k] += LearningRate * (errorPred*m.Q[i][k] - Regularization*m.P[u][k])
	}

	// 2. Ítems: Fine-Grained Locking (Alta popularidad = alto riesgo de colisión)
	m.ItemMutexes[i].Lock()
	for k := 0; k < LatentFeatures; k++ {
		m.Q[i][k] += LearningRate * (errorPred*m.P[u][k] - Regularization*m.Q[i][k])
	}
	m.ItemMutexes[i].Unlock()
}

// Utilidades para inicializar matrices y cargar CSV
func inicializarModelo(nUsers, nItems int) *Modelo {
	m := &Modelo{
		P:           make([][]float64, nUsers),
		Q:           make([][]float64, nItems),
		ItemMutexes: make([]sync.Mutex, nItems),
	}
	for i := 0; i < nUsers; i++ {
		m.P[i] = make([]float64, LatentFeatures)
		for k := range m.P[i] {
			m.P[i][k] = rand.Float64()
		}
	}
	for i := 0; i < nItems; i++ {
		m.Q[i] = make([]float64, LatentFeatures)
		for k := range m.Q[i] {
			m.Q[i][k] = rand.Float64()
		}
	}
	return m
}

func cargarCSV(ruta string, total int) []Interaction {
	file, _ := os.Open(ruta)
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Read() // Omitir encabezados

	interacciones := make([]Interaction, 0, total)
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		u, _ := strconv.Atoi(record[0])
		i, _ := strconv.Atoi(record[1])
		r, _ := strconv.ParseFloat(record[2], 64)
		interacciones = append(interacciones, Interaction{UserIndex: u, ItemIndex: i, Rating: r})
	}
	return interacciones
}