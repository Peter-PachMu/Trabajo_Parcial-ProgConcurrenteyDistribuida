# Trabajo_Parcial-ProgConcurrenteyDistribuida
## Sistema de Recomendación de Consumo Sostenible en Retail (ODS 12)

Este repositorio contiene la fase de preparación de datos (ETL) y el modelo de Filtrado Colaborativo desarrollado para predecir patrones de compra utilizando programación concurrente.

## Origen de los Datos

Para ejecutar el código de limpieza y reproducir la matriz de interacciones, debes descargar el dataset publico desde Kaggle:

🔗 [Instacart Market Basket Analysis Dataset](https://www.kaggle.com/datasets/psparks/instacart-market-basket-analysis/data)

### Instrucciones de Configuración
1. Descarga el archivo comprimido desde el enlace proporcionado.
2. Extrae los siguientes archivos y colócalos en el directorio principal del proyecto:
   * `orders.csv`
   * `order_products__prior.csv`
   * `products.csv`
3. Ejecuta el notebook de preparación de datos para aplicar el filtrado iterativo (k-core).
4. El script generará los archivos `instacart_matriz_limpia.csv`, `productos_mapeados.csv` y `matriz_meta.json`.

## Arquitectura
* **Python (Pandas):** Limpieza de ruido, mitigación de Cold Start* y mapeo categórico. El dataset final procesado supera los 13 millones de registros.
* **Go:** Implementación del algoritmo de factorización de matrices utilizando concurrencia nativa (*Worker Pools* y *Fine-Grained Locking*).

## Ejecución del Modelo Concurrente (Go)
Para evaluar el rendimiento y calcular el *Speedup* del algoritmo, asegúrate de haber generado el archivo `instacart_matriz_limpia.csv` y colócalo en la raíz del proyecto. Luego, ejecuta el siguiente comando en la terminal:
```bash
go run main.go
```
El programa ejecutará automáticamente una batería de 10 pruebas, alternando entre el procesamiento secuencial y el concurrente (implementado mediante Worker Pools, Fine-Grained Locking y procesamiento por lotes o Chunking). Al finalizar, imprimirá en la consola una tabla estadística con la Media Recortada de los tiempos y el Speedup estabilizado del sistema.


