# Informe Técnico de GAPs — Sistema de Recomendación SGD concurrente en Go

**Alcance:** `main.go`, `matriz_meta.json`, formato de `matriz_limpia.csv` / `productos_mapeados.csv`, y artefactos de verificación formal (`pan.m`, derivado de `modelo.pml`).
**Dataset:** 37 373 usuarios · 1 301 ítems · 1 711 253 interacciones (densidad 3.52 %).
**Configuración actual:** K = 10, α = 0.0005, λ = 0.05, 12 workers, lotes de 50 000, 10 épocas.

> **Nota de alcance:** el análisis de la capa Promela se basa en el código generado `pan.m` (que refleja fielmente las transiciones de `modelo.pml`: 1 proceso `Worker`, 3 ítems, `locks[3]`, `in_critical_section[3]` y un `assert(in_critical_section[item_id]==1)`). No se auditó el código C generado (`pan.c`, `pan.h`, `pan.p`, `pan.t`, `pan.b`) más allá de su presencia en el repositorio.

---

## 0. Resumen Ejecutivo

| # | GAP | Área | Severidad |
|---|-----|------|-----------|
| C-1 | Data race real en `P[u]` (y lecturas sin sincronizar de `Q[i]`) — "lock-free" ≠ "race-free" en el modelo de memoria de Go | Concurrencia | **Crítica** |
| C-2 | La versión concurrente **no es numéricamente equivalente** a la secuencial (falta `tempPu`) | Concurrencia / Calidad | **Alta** |
| C-3 | Granularidad del pool: 35 lotes para 12 workers → desbalance de carga | Concurrencia | Media |
| C-4 | *False sharing* en `[]sync.Mutex` y *pointer chasing* en `[][]float64` | Concurrencia | Media |
| C-5 | Workers sin `recover`, sin `context`, sin propagación de errores | Concurrencia | Media |
| C-6 | Verificación formal Promela no cubre las propiedades críticas del diseño | Concurrencia | Alta |
| Q-1 | Monolito en `main()`, constantes mágicas, sin tests | Calidad | Alta |
| Q-2 | Errores ignorados sistemáticamente (`_`) y truncamiento silencioso del CSV | Calidad | **Crítica** |
| Q-3 | Metodología de benchmark sesgada | Calidad | Media |
| Q-4 | Artefactos generados de SPIN versionados en el repo | Calidad | Baja |
| S-1 | Índices del CSV sin validar → *panic* / DoS | Seguridad | **Alta** |
| S-2 | Confianza ciega en `matriz_meta.json` (`make` con tamaño controlado externamente) | Seguridad | Alta |
| S-3 | Rutas relativas fijas, sin validación ni saneamiento | Seguridad | Media |
| S-4 | `NaN`/`Inf` aceptados por `ParseFloat` → envenenamiento del modelo | Seguridad | Media |
| S-5 | Privacidad (Ley 29733 – Perú), *shilling attacks*, inyección en salida | Seguridad | Media |
| S-6 | Cadena de suministro: sin `go.sum` verificado, sin `govulncheck`, código C generado en repo | Seguridad | Baja |

---

## 1. GAPs en Patrones de Concurrencia

### C-1 · Data race real en la actualización "lock-free" de usuarios — **Crítica**

**Evidencia (`actualizarFactoresConcurrente`):**

```go
// 1. Usuarios: Lock-Free (baja probabilidad de colisión)
for k := 0; k < LatentFeatures; k++ {
    m.P[u][k] += LearningRate * (errorPred*m.Q[i][k] - Regularization*m.P[u][k])
}
```

**Problemas:**

1. **Escritura concurrente sin sincronización sobre `P[u][k]`.** Si dos workers procesan interacciones del mismo usuario (lotes distintos, o el CSV no está ordenado por usuario), existe un *data race* según el Go Memory Model. En Go un race **no es simplemente "una actualización perdida"**: es comportamiento formalmente indefinido para el programa, y el compilador puede reordenar/cachear en registros. La justificación "baja probabilidad de colisión" no es una garantía.
2. **Lecturas sin lock de `Q[i][k]`** al calcular `prediccion` y `errorPred` mientras otro worker escribe `Q[i]` bajo mutex. El mutex solo protege a escritores contra escritores; los lectores no participan → *read/write race*.
3. **Lectura de `P[u][k]` dentro de la sección crítica de ítems** (`errorPred*m.P[u][k]`): lee un valor de `P` que el propio worker acaba de modificar (y que otro worker puede estar modificando).
4. La **suposición de asimetría** (usuarios con baja colisión, ítems con alta) depende del orden de los datos. Con lotes contiguos de 50 000 filas, si el CSV está ordenado por usuario la suposición es válida por accidente; si no, no lo es. Es una propiedad *implícita del dataset*, no del diseño.

**Impacto:** resultados no reproducibles y posible degradación de convergencia.

**Remediación:**

- Ejecutar de inmediato `go run -race .` y `go test -race` (hoy no existen tests).
- Elegir **una** de estas estrategias explícitamente y documentarla:
  - **Hogwild! formal:** aceptar races benignos pero implementarlos con `sync/atomic` sobre `uint64` (`math.Float64bits`) para evitar comportamiento indefinido.
  - **Partición por bloques (DSGD / FPSGD):** dividir la matriz en una rejilla P×P de bloques; en cada sub-época los workers procesan bloques que **no comparten ni filas ni columnas** → **cero locks y cero races**.

---

### C-2 · Divergencia semántica entre secuencial y concurrente — **Alta**

En la versión secuencial se preserva el valor previo del factor de usuario:

```go
tempPu := m.P[u][k]
m.P[u][k] += LearningRate * (errorPred*m.Q[i][k] - Regularization*m.P[u][k])
m.Q[i][k] += LearningRate * (errorPred*tempPu - Regularization*m.Q[i][k])
```

En la concurrente, `Q` se actualiza usando el `P[u][k]` **ya modificado**. Son dos algoritmos distintos (SGD simultáneo vs. Gauss-Seidel secuencial). Por tanto:

- El *speedup* reportado compara **trabajos no numéricamente equivalentes**.
- No hay forma de afirmar que la versión concurrente converge al mismo óptimo.

**Remediación:** capturar `pu := make([]float64, K)` (o un array `[K]float64` en stack para evitar allocs) antes de modificar `P[u]`, y usar `pu[k]` para actualizar `Q[i]`. Validar equivalencia comparando RMSE final de ambas versiones con la misma semilla.

---

### C-3 · Granularidad del Worker Pool y balanceo de carga — **Media**

- `numWorkers := 12` está **hardcodeado**; debería derivarse de `runtime.GOMAXPROCS(0)` o ser configurable.
- `1 711 253 / 50 000 ≈ 35 lotes` para 12 workers → 2 rondas completas + 1 ronda de 11 lotes, con el último lote de ~11 253 filas (22 % de un lote normal). El **efecto cola** limita el speedup máximo teórico; el tamaño de lote no está justificado empíricamente.
- El canal está dimensionado a `numChunks`, por lo que el productor nunca bloquea (no hay deadlock), pero **no hay *backpressure*** ni streaming: se carga el dataset completo en memoria antes de empezar.

**Remediación:** barrido de `chunkSize` (1 k – 100 k) y `numWorkers`; apuntar a ≥ 4–8 unidades de trabajo por worker; o usar un **índice atómico compartido** (`atomic.AddInt64(&next, chunk)`) en lugar de un canal, eliminando por completo la sincronización del canal.

---

### C-4 · *False sharing* y localidad de memoria — **Media**

- `ItemMutexes []sync.Mutex`: cada `sync.Mutex` ocupa 8 bytes ⇒ **8 mutex por línea de caché de 64 B**. Workers que bloquean ítems *distintos pero adyacentes* invalidan mutuamente su línea de caché (*false sharing*).
- `P [][]float64` / `Q [][]float64` son *slices de slices*: 37 373 + 1 301 allocaciones separadas, con *pointer chasing* y mala localidad espacial. Además aumentan presión sobre el GC.
- El mutex se adquiere para solo **10 multiplicaciones-suma**; el costo de `Lock/Unlock` (decenas de ns, más si hay contención) puede igualar o superar al trabajo útil.

**Remediación:**

```go
type Modelo struct {
    P []float64 // len = nUsers*K, acceso P[u*K : (u+1)*K]
    Q []float64 // len = nItems*K
    locks []paddedMutex
}
type paddedMutex struct {
    sync.Mutex
    _ [56]byte // padding a 64 B
}
```

Alternativas: *lock striping* (p. ej. 64–256 mutex con `i % stripes`), `float32` (mitad de memoria y ancho de banda), o eliminar locks vía partición por bloques (C-1).

---

### C-5 · Ciclo de vida de goroutines: sin `recover`, `context` ni errores — **Media**

- Un `panic` dentro de un worker (p. ej. índice fuera de rango por dato inválido, ver S-1) **termina todo el proceso**; no hay `defer recover()`.
- No hay `context.Context` para cancelar el entrenamiento (timeout, SIGTERM en un pod, etc.).
- Los workers no devuelven errores: no se usa `errgroup.Group`.
- No hay fugas de goroutines hoy (el canal se cierra y `wg.Wait()` espera), pero el diseño es **frágil**: si se añade una salida temprana en el productor sin cerrar el canal, los 12 workers quedarían bloqueados indefinidamente en `range canalLotes`.

**Remediación:** `golang.org/x/sync/errgroup` con `WithContext`, `defer func(){ if r := recover(); ... }()` en cada worker, y productor con `select { case canal <- lote: case <-ctx.Done(): return }`.

---

### C-6 · La verificación formal (Promela/SPIN) no respalda las afirmaciones del código — **Alta**

El comentario en `main.go` afirma: *"Actualización asimétrica demostrada en el paper A-ASGD y simulada en Promela"*. Del modelo generado (`pan.m`) se observa:

| Aspecto | Lo que modela `modelo.pml` | Lo que el código Go realmente hace |
|---|---|---|
| Procesos | Un tipo `Worker` | 12 goroutines + productor |
| Recurso compartido | Solo `locks[item]` y `in_critical_section[item]` | `P[u]` sin lock, `Q[i]` con lock, **lecturas sin lock** |
| Propiedad verificada | Exclusión mutua: `assert(in_critical_section[item_id]==1)` | Además se necesita: ausencia de *data races*, *deadlock*, terminación del canal, *lost updates* |
| Datos | 3 ítems (`item_id` ∈ {0,1,2}) | 1 301 ítems, 37 373 usuarios |
| Usuarios lock-free | **No modelado** | Núcleo de la tesis de diseño |

Además, la propiedad de exclusión mutua con un `lock` booleano busy-wait es **correcta por construcción** y no aporta evidencia sobre la parte riesgosa (los accesos sin lock). 

**Remediación:**
- Modelar `P[u]` (escritura sin lock) y las **lecturas sin lock de `Q[i]`**; añadir un `assert` de *lost update* (contador de actualizaciones esperado vs. observado).
- Aumentar a ≥ 3 workers y ≥ 2 usuarios por ítem para que existan intercalados relevantes.
- Añadir LTL para progreso (`[]<>` terminación) y verificar ausencia de *deadlock* con `-DSAFETY`/`-a`.
- Complementar con `go test -race`, que es la verificación que sí aplica al código real.

---

## 2. GAPs en Calidad de Código

### Q-1 · Modularidad, acoplamiento y ausencia de pruebas — **Alta**

- **`main.go` monolítico (~330 líneas)** mezcla: lectura de metadatos, carga de datos, entrenamiento, benchmark, estadística, predicción e impresión. Viola SRP.
- **Todo en `package main`**: imposible de importar, testear o reutilizar (p. ej. desde una API HTTP/gRPC).
- **Hiperparámetros como `const`** (`LatentFeatures`, `LearningRate`, `Regularization`) y *magic numbers* (`numWorkers = 12`, `repeticiones = 10`, `chunkSize = 50000`, `usuarioPrueba = 0`, top 5). No hay `flag`, variables de entorno ni struct de configuración. 
- **Estado compartido sin encapsulación:** `Modelo` expone `P`, `Q`, `ItemMutexes` como campos públicos; el candado vive en el modelo aunque la versión secuencial no lo use.
- **Cero pruebas:** sin `*_test.go`, sin `testing.B` (el benchmark manual con `time.Now()` debería migrar a `go test -bench`), sin `go vet`/`staticcheck`/`golangci-lint` en CI.
- **Nomenclatura mixta** español/inglés (`EntrenarSecuencial`, `Interaction`, `actualizarFactores`), y funciones exportadas sin comentario GoDoc.
- **Sin persistencia del modelo:** P y Q se re-entrenan en cada ejecución; no hay `Save/Load` (p. ej. `encoding/gob`).

---

### Q-2 · Manejo de errores deficiente y truncamiento silencioso — **Crítica**

| Ubicación | Defecto | Consecuencia |
|---|---|---|
| `main`: `json.NewDecoder(metaFile).Decode(&meta)` | Error ignorado | JSON corrupto ⇒ `meta` en ceros ⇒ modelo de tamaño 0 ⇒ *panic* tardío e incomprensible |
| `cargarCSV`: `file, _ := os.Open(ruta)` | Error ignorado | Archivo inexistente ⇒ `file == nil`; el primer `reader.Read()` devuelve error, el bucle `break` y se devuelve **un slice vacío sin avisar** |
| `cargarCSV`: `reader.Read()` del encabezado | Error ignorado | Encabezado ausente se interpreta como dato |
| `cargarCSV`: `strconv.Atoi` / `ParseFloat` con `_` | Fallo ⇒ valor `0` | Filas corruptas se convierten silenciosamente en `(user 0, item 0, rating 0)` |
| `cargarCSV`: `if err != nil { break }` | No distingue `io.EOF` de errores de parseo | Un error de formato a mitad del archivo **trunca el dataset sin avisar** |
| `cargarCSV`: `record[0..2]` | Sin validar `len(record)` | `panic: index out of range` |
| `cargarCatalogo`: `strconv.Atoi(linea[0])` y `linea[3]` | Error ignorado / sin validar columnas | ID `0` falso o *panic* |

**Remediación:** funciones que devuelvan `(T, error)`, envolver con `fmt.Errorf("...: %w", err)`, distinguir `errors.Is(err, io.EOF)`, usar `reader.FieldsPerRecord = 3`, `reader.ReuseRecord = true` (menos allocs), y **fallar rápido**.

---

### Q-3 · Metodología de benchmark — **Media**

- **Orden fijo secuencial → concurrente** en cada iteración: sesgo por caché, *thermal throttling* y estado del GC. Alternar el orden o aislar procesos.
- **`time.Since(...).Milliseconds()` trunca a entero** y luego se convierte a `float64`; usar `.Seconds()` o `float64(d)/1e6`.
- Los `sort.Float64s` ordenan `tiemposSecuencial`, `tiemposConcurrente` y `speedups` **de forma independiente**: se pierde el emparejamiento por iteración y la media recortada de speedups **no es igual** al cociente de las medias recortadas (se reportan como si lo fueran).
- Sin desviación estándar, intervalo de confianza ni mediana; 10 repeticiones con recorte de 1 por extremo es estadísticamente pobre.
- Sin **calentamiento** (*warm-up*) ni `runtime.GC()` previo a cada medición.
- `rand.Float64()` global y sin semilla fija ⇒ cada iteración parte de un estado distinto y secuencial/concurrente **no comparten inicialización**. Usar `rand.New(rand.NewSource(seed))` y clonar el modelo inicial.

---

### Q-4 · Higiene del repositorio — **Baja**

- `pan.b`, `pan.c`, `pan.h`, `pan.m`, `pan.p`, `pan.t` son **artefactos generados** por `spin -a modelo.pml`. Deben regenerarse en CI y excluirse con `.gitignore`; conservar solo `modelo.pml` y un script (`make verify`).
- Los CSV de 1.7 M de filas deberían estar fuera de Git (Git LFS, DVC o almacenamiento de objetos).
- Falta `README`, `Makefile`, `LICENSE`, `.golangci.yml`, pipeline CI (`go vet`, `-race`, `staticcheck`, `govulncheck`).

---

## 3. GAPs en Seguridad

### S-1 · Índices de entrada sin validar → *panic* / Denegación de Servicio — **Alta**

`UserIndex` e `ItemIndex` se usan directamente como índices en `m.P[u]`, `m.Q[i]` y `m.ItemMutexes[i]` sin verificar `0 ≤ idx < N`. Un CSV manipulado (o simplemente corrupto) con un índice fuera de rango provoca `panic: index out of range`. Al ocurrir **dentro de una goroutine** (sin `recover`) el proceso completo termina.

**Remediación:** validar en la carga y mantener `recover` como defensa en profundidad.

---

### S-2 · Confianza ciega en `matriz_meta.json` — **Alta**

- `make([]Interaction, 0, total)` con `total = meta.NInteractions`: un valor negativo provoca `panic`, uno enorme (p. ej. 10¹⁰) provoca **agotamiento de memoria** (OOM-kill) antes de leer una sola fila.
- `inicializarModelo(meta.NUsers, meta.NItems)` reserva `NUsers × K × 8` bytes sin tope: metadatos alterados ⇒ asignación arbitraria.

**Remediación:** límites máximos configurables (`maxUsers`, `maxItems`, `maxInteractions`), `http.MaxBytesReader`/`io.LimitReader` si los archivos llegan por red, y SHA-256 de los insumos.

---

### S-3 · Manipulación de archivos y rutas — **Media**

- Rutas **relativas literales** (`"matriz_limpia.csv"`, `"productos_mapeados.csv"`, `"matriz_meta.json"`): el comportamiento depende del *working directory*.
- Hoy no hay entrada de usuario en las rutas, pero la evolución natural abre **path traversal** (`../../etc/passwd`) si no se aplica `filepath.Clean`.
- Sin **límite de tamaño** de archivo; `reader.ReadAll()` en `cargarCatalogo` carga todo en memoria.
- Apertura con `os.Open` sin comprobación de errores.

---

### S-4 · Valores numéricos hostiles: `NaN`, `±Inf` y divergencia — **Media**

`strconv.ParseFloat` acepta `"NaN"`, `"Inf"`, `"-Inf"` y valores como `1e308`. Una sola fila con estos valores propaga `NaN` a `P[u]` y `Q[i]` y, por contagio, a **todos los factores asociados**, produciendo recomendaciones arbitrarias. Combinado con el race de C-1, no es detectable a posteriori.

**Remediación:** `math.IsNaN/IsInf` en carga, rango de rating permitido, *gradient clipping*, y verificación `NaN` post-época.

---

### S-5 · Privacidad, integridad del modelo y salida — **Media**

- **Datos personales:** en producción, `user_index` + historial de compras constituyen datos personales sujetos a la **Ley N.° 29733 (Protección de Datos Personales, Perú)** y su reglamento. Se requieren seudonimización/minimización.
- **Envenenamiento del modelo (*shilling / data-poisoning attacks*):** el filtrado colaborativo es vulnerable a perfiles falsos que inflen un producto. 
- **Inyección en la salida:** `Nombre` y `Marca` proceden de un CSV y se imprimen sin sanitizar: secuencias de escape ANSI pueden manipular la terminal.
- **Reproducibilidad / aleatoriedad:** `math/rand` no es criptográfico (aceptable para inicialización de pesos), pero sin semilla fija no hay trazabilidad para auditoría.

---

### S-6 · Cadena de suministro y artefactos de verificación — **Baja**

- Solo se usa la biblioteca estándar (**positivo**: superficie mínima), pero no se evidencia `go.sum`, fijación de versión de toolchain (`toolchain` en `go.mod`) ni escaneo con **`govulncheck`**. Añadir `go mod verify` y SBOM (`cyclonedx-gomod`) en CI.
- Se versiona **código C autogenerado** (`pan.c`) que suele compilarse con flags permisivos. Regenerarlo en CI desde `modelo.pml`.

---

## 4. Hoja de Ruta Priorizada

| Prioridad | Acción | GAPs cubiertos | Esfuerzo |
|---|---|---|---|
| **P0** | Correr `go run -race`; corregir `tempPu`; propagar errores en `cargarCSV`/`main`; validar rangos de índices y metadatos | C-1, C-2, Q-2, S-1, S-2 | Bajo |
| **P1** | Semilla fija y modelo inicial clonado para comparar la convergencia secuencial/concurrente de manera limpia | Q-3 | Medio |
| **P2** | Rediseñar a partición por bloques (DSGD) o aplanar matrices + mutex con padding; barrido de `chunkSize`/workers | C-1, C-3, C-4 | Medio-Alto |
| **P2** | Reestructurar en paquetes; tests unitarios, `-race` y `testing.B`; CI con `vet`, `staticcheck`, `govulncheck` | Q-1, Q-4, S-6 | Medio |
| **P2** | `errgroup` + `context` + `recover` en workers | C-5 | Bajo |
| **P3** | Ampliar modelo Promela (lecturas sin lock, usuarios, ≥3 workers, LTL) o reemplazar la afirmación por evidencia de `-race` | C-6 | Medio |
| **P3** | Manejo seguro de rutas (`os.OpenRoot`), límites de tamaño, checksums, filtrado de `NaN/Inf`, política de privacidad Ley 29733 | S-3, S-4, S-5 | Medio |

---

## 5. Fragmento de Referencia (corrección mínima de C-2 y S-1)

```go
func (m *Modelo) paso(obs Interaction) {
    u, i := obs.UserIndex, obs.ItemIndex
    pu := m.P[u*K : (u+1)*K]
    qi := m.Q[i*K : (i+1)*K]

    var pred float64
    for k := 0; k < K; k++ {
        pred += pu[k] * qi[k]
    }
    e := obs.Rating - pred

    for k := 0; k < K; k++ {
        puk := pu[k] // valor previo, idéntico al secuencial
        pu[k] += LR * (e*qi[k] - Lambda*puk)
        qi[k] += LR * (e*puk - Lambda*qi[k])
    }
}

func validar(obs Interaction, nU, nI int) error {
    switch {
    case obs.UserIndex < 0 || obs.UserIndex >= nU:
        return fmt.Errorf("user_index fuera de rango: %d", obs.UserIndex)
    case obs.ItemIndex < 0 || obs.ItemIndex >= nI:
        return fmt.Errorf("item_index fuera de rango: %d", obs.ItemIndex)
    case math.IsNaN(obs.Rating) || math.IsInf(obs.Rating, 0):
        return fmt.Errorf("rating inválido: %v", obs.Rating)
    }
    return nil
}
```

*(Con partición por usuario o por bloques, este mismo `paso` es seguro sin locks.)*

---

## 6. Conclusión

La implementación demuestra correctamente el **patrón** Worker Pool con *batching* y un uso razonable de `sync.Mutex` por ítem. Sin embargo, las brechas de mayor impacto que quedan pendientes son: **(1)** la versión "lock-free" para usuarios constituye un *data race* real en Go y no está respaldada por la verificación Promela, **(2)** las versiones secuencial y concurrente **no son numéricamente equivalentes** (divergencia en la actualización matemática), y **(3)** la carga de datos ignora errores y no valida entradas, lo que produce truncamientos silenciosos y *panics* explotables. Resolver el bloque P0/P1 de la hoja de ruta convierte el prototipo en una base técnicamente defendible.