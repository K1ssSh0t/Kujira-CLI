✦ Perfecto. Ahora leeré el contenido de main.go y los archivos del extractor para entender la lógica de la aplicación.
╭───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ ✓  ReadManyFiles Will attempt to read and concatenate files using patterns:                                                                                           │
│    main.go`, `extractors/main.go`, `extractors/streamtape.go`, `extractors/voe.go                                                                                     │
│     (within target directory:                                                                                                                                         │
│    /home/user/code-projects/Kujira-CLI                                                                                                                                │
│    ) . Excluding: patterns like                                                                                                                                       │
│    **/node_modules/**`, `**/.git/**...`. File encoding: utf-8. Separator: "--- path/to/file.ext ---".                                                                 │
│                                                                                                                                                                       │
│    ### ReadManyFiles Result (Target Dir: `/home/user/code-projects/Kujira-CLI`)                                                                                       │
│                                                                                                                                                                       │
│    Successfully read and concatenated content from **4 file(s)**.                                                                                                     │
│                                                                                                                                                                       │
│    **Processed Files:**                                                                                                                                               │
│    - `extractors/main.go`                                                                                                                                             │
│    - `extractors/streamtape.go`                                                                                                                                       │
│    - `extractors/voe.go`                                                                                                                                              │
│    - `main.go`                                                                                                                                                        │
╰───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
✦ Okay, he revisado el código. Aquí está mi análisis y el plan para implementar el caché y el historial de episodios.

  Análisis del Código

   * Punto Central de Peticiones: La mayoría de las solicitudes a la API se realizan a través de las funciones Fetch... en extractors/main.go y los métodos get... en
     main.go. Estos son los lugares ideales para implementar el caché.
   * Flujo de Reproducción: La función playVideo en main.go se llama justo antes de reproducir un video. Este es el punto perfecto para registrar un episodio en el
     historial.
   * Estructura de Datos: Las estructuras de datos para Anime, Episode, etc., ya están definidas en extractors/main.go. Podemos reutilizarlas.

  Plan de Implementación

  1. Sistema de Caché

  Para evitar peticiones frecuentes, propongo un sistema de caché simple basado en archivos JSON.

   1. Crear un directorio de caché: Un directorio como .kujira_cache en el directorio de inicio del usuario para almacenar los archivos de caché.
   2. Modificar las funciones de "fetch":
       * Antes de hacer una petición HTTP en funciones como FetchLatestEpisodes, FetchEpisodeDetail, FetchAnime, y FetchSearch, se comprobará si existe un archivo de caché
          válido y reciente para esa petición.
       * El nombre del archivo de caché puede ser un hash del URL de la petición para asegurar que sea único.
       * Si existe un archivo de caché y no ha expirado (p. ej., tiene menos de 1 hora), se leerán los datos desde el archivo en lugar de hacer la petición a la red.
       * Si no hay caché o está expirado, se hará la petición a la red y el resultado se guardará en un nuevo archivo de caché con la fecha y hora actual.

  2. Historial de Episodios

   1. Archivo de Historial: Se creará un archivo history.json en el directorio de inicio del usuario para almacenar una lista de los episodios vistos. Cada entrada en el
      historial guardará información clave como el título del anime, número de episodio, y la URL para acceder a él de nuevo.
   2. Guardar en el Historial: Se modificará la función playVideo en main.go. Justo después de que un episodio se inicie correctamente, se añadirá una entrada al archivo
      history.json.
   3. Menú de Historial:
       * Se añadirá una nueva opción "Historial de episodios" en el mainMenu.
       * Al seleccionar esta opción, se leerá el archivo history.json y se mostrará una lista de los episodios vistos.
       * El usuario podrá seleccionar un episodio del historial para volver a ver los detalles de sus servidores y reproducirlo.

  ¿Te parece bien este plan? Si estás de acuerdo, empezaré con la implementación del sistema de caché.
