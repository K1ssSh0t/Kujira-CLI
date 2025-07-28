import * as cheerio from 'cheerio';

// Función para decodificar base64
function base64Decode(input: string): string {
    return Buffer.from(input, 'base64').toString('utf-8');
}

// Función de descifrado específica para Voe
function decryptF7(p8: string): Record<string, any> {
    const vF = p8.replace(/[A-Za-z]/g, c =>
        String.fromCharCode(((c.charCodeAt(0) - (c <= 'Z' ? 65 : 97) + 13) % 26 + (c <= 'Z' ? 65 : 97))
        ));
    const vF3 = vF.replace(/[@$^^~@%?*~!!#&]/g, '_').replace(/_/g, '');
    const vF4 = base64Decode(vF3);
    const vF5 = Array.from(vF4, ch => String.fromCharCode(ch.charCodeAt(0) - 3)).join('');
    const vF6 = vF5.split('').reverse().join('');
    return JSON.parse(base64Decode(vF6));
}

// Función principal para extraer el MP4
export async function getVoeMp4(url: string): Promise<string | null> {
    const res = await fetch(url);
    let html = await res.text();

    // Manejar redirección si existe
    const redirectMatch = /window\.location\.href\s*=\s*'([^']+)';/.exec(html);
    if (redirectMatch?.[1]) {
        const newUrl = redirectMatch[1];
        const newRes = await fetch(newUrl);
        html = await newRes.text();
    }

    const $ = cheerio.load(html);
    const scriptContent = $('script[type="application/json"]').html()?.trim() || '';
    const encodedMatch = scriptContent.match(/\["(.+)"\]/);

    if (!encodedMatch?.[1]) return null;

    const decrypted = decryptF7(encodedMatch[1]);
    return decrypted['direct_access_url'] || null;
}