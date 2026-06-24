/**
 * Google Slides MCP Server
 * Configuration — lecture et validation des variables d'environnement
 */

export interface GSlidesConfig {
  serviceAccountKey: string;
  templateId: string | undefined;
  baseUrl: string;
  driveBaseUrl: string;
  timeout: number;
  maxRetries: number;
}

export function getConfig(): GSlidesConfig {
  const serviceAccountKey = process.env.GOOGLE_SERVICE_ACCOUNT_KEY;

  if (!serviceAccountKey) {
    throw new Error(
      'GOOGLE_SERVICE_ACCOUNT_KEY is required. ' +
      'Encode your Service Account JSON key with: base64 -i sa-key.json | tr -d \'\\n\'\n' +
      'Then configure it via: oc gslides setup'
    );
  }

  // Validation que le contenu est un base64 décodable en JSON valide
  try {
    const decoded = Buffer.from(serviceAccountKey, 'base64').toString('utf8');
    const parsed = JSON.parse(decoded);
    if (!parsed.type || parsed.type !== 'service_account') {
      throw new Error('Le JSON décodé n\'est pas une clé Service Account valide (champ "type" manquant ou incorrect)');
    }
  } catch (err) {
    throw new Error(
      `GOOGLE_SERVICE_ACCOUNT_KEY invalide : impossible de décoder le JSON Service Account. ` +
      `Vérifier que la valeur est bien encodée en base64.\nDétail : ${err instanceof Error ? err.message : String(err)}`
    );
  }

  const templateId = process.env.GOOGLE_SLIDES_TEMPLATE_ID || undefined;

  const rawTimeout = parseInt(process.env.GOOGLE_SLIDES_TIMEOUT ?? '30000', 10);
  const timeout = isNaN(rawTimeout) || rawTimeout <= 0 ? 30000 : rawTimeout;

  const rawRetries = parseInt(process.env.GOOGLE_SLIDES_MAX_RETRIES ?? '2', 10);
  const maxRetries = isNaN(rawRetries) || rawRetries < 0 ? 2 : rawRetries;

  return {
    serviceAccountKey,
    templateId,
    baseUrl: 'https://slides.googleapis.com/v1',
    driveBaseUrl: 'https://www.googleapis.com/drive/v3',
    timeout,
    maxRetries,
  };
}
