/**
 * Utilitaires partagés pour les tests gitlab-mcp
 */

/** Crée une erreur Axios simulée pour les tests */
export function makeAxiosError(
  options: {
    code?: string;
    status?: number;
    message?: string;
    timeout?: number;
  } = {}
): any {
  const err: any = new Error(options.message || 'axios error');
  err.isAxiosError = true;
  err.code = options.code;
  err.config = { timeout: options.timeout ?? 30000 };
  if (options.status !== undefined) {
    err.response = {
      status: options.status,
      data: {},
    };
  }
  return err;
}
