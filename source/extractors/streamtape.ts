export async function extractStreamTapeUrl(pageUrl: string): Promise<string> {
    const res = await fetch(pageUrl, {
        headers: {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3'
        }
    }
    );
    // console.log("Fetching StreamTape page:", pageUrl, res.status);
    const html = await res.text();

    const scriptMatches = html.match(/<script[^>]*>([\s\S]*?)<\/script>/g);
    let jsLine: string | null = null;

    if (scriptMatches) {
        for (const script of scriptMatches) {
            const lines = script.split('\n');
            for (const line of lines) {
                if (line.includes("botlink').innerHTML")) {
                    jsLine = line.trim();
                    break;
                }
            }
            if (jsLine) break;
        }
    }

    if (!jsLine) {
        throw new Error("No se encontró el script con 'botlink'.");
    }

    // Extraer la expresión JS (después de ").innerHTML = ...")
    const exprMatch = jsLine.match(/innerHTML\s*=\s*(.+);/);
    if (!exprMatch || !exprMatch[1]) {
        throw new Error("No se pudo extraer la expresión de JS.");
    }

    const jsExpression = exprMatch[1]; // Ej: `"//domain.com/stream123?token=abc" + ...`

    const resultUrl = evalUrlScript(jsExpression);

    if (!resultUrl) {
        throw new Error("No se pudo evaluar la URL.");
    }

    return `https:${resultUrl}&stream=1`;
}

// ✅ Evalúa una expresión JavaScript de forma segura y retorna `url`
// function evalUrlScript(jsExpression: string): string {
//     try {
//         // Evalúa la expresión dentro de una función segura
//         'use strict'; 4
//         console.log("Evaluando expresión JS:", jsExpression);
//         const url = Function(`\"use strict\"; return (${jsExpression});`)();
//         if (typeof url !== 'string') throw new Error("El resultado no es una cadena.");
//         console.log("URL evaluada:", url);
//         return url;
//     } catch (e) {
//         throw new Error("Error al evaluar el JS: " + (e as Error).message);
//     }
// }
function evalUrlScript(jsExpression: string): string {
    // Soporta expresiones del tipo: 'a' + ('b').substring(n)
    // Ejemplo: '//streamtape.co'+ ('xyzam/get_video?...').substring(4)
    // Resultado esperado: "//streamtape.com/get_video?..."
    const plusSplit = jsExpression.split('+').map(s => s.trim());

    let result = '';
    for (let part of plusSplit) {
        // Quita comillas simples o dobles
        if (/^['"]/.test(part)) {
            result += part.slice(1, -1);
        } else {
            // Busca .substring(N)
            const substringMatch = part.match(/^\((['"])(.*?)\1\)\.substring\((\d+)\)$/);
            if (substringMatch) {
                const str = substringMatch[2];
                const start = parseInt(substringMatch[3] as string, 10);
                result += str!.substring(start);
            } else {
                // Si no hay substring, intenta parsear como string simple
                const strMatch = part.match(/^\((['"])(.*?)\1\)$/);
                if (strMatch) {
                    result += strMatch[2];
                } else {
                    throw new Error("No se pudo parsear la expresión JS: " + part);
                }
            }
        }
    }
    return result;
}


// // 🔍 Ejemplo de uso:
// (async () => {
//     try {
//         const directUrl = await extractStreamTapeUrl("https://streamtape.com/e/XgK9zqvq6BTDqzx/");
//         console.log("✅ Enlace directo:", directUrl);
//     } catch (err) {
//         console.error("❌ Error:", (err as Error).message);
//     }
// })();