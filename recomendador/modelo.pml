#define NUM_ITEMS 3
#define NUM_WORKERS 2

// Arreglo de candados (simulando los sync.Mutex por producto)
bool locks[NUM_ITEMS];

// Variable para verificar que solo un worker entra a la vez
byte in_critical_section[NUM_ITEMS];

active [NUM_WORKERS] proctype Worker() {
    byte item_id;
    
    do
    :: true ->
        // 1. El worker elige un producto aleatorio para procesar
        if
        :: item_id = 0;
        :: item_id = 1;
        :: item_id = 2;
        fi;

        // 2. Adquisición del candado (Lock) de forma atómica
        atomic {
            locks[item_id] == false; // Espera hasta que el candado este libre
            locks[item_id] = true;   // Bloquea el producto
        }

        // --- INICIO DE LA SECCIÓN CRÍTICA (Actualizacion de gradientes) ---
        in_critical_section[item_id]++;
        
        // VERIFICACIÓN FORMAL: Si hay más de 1 worker aqui, el assert falla
        assert(in_critical_section[item_id] == 1); 
        
        in_critical_section[item_id]--;
        // --- FIN DE LA SECCIÓN CRÍTICA ---

        // 3. Liberación del candado (Unlock)
        locks[item_id] = false;
    od
}