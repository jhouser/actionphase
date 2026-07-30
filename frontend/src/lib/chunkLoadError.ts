export function isChunkLoadError(error: Error): boolean {
  return (
    error.name === 'ChunkLoadError' ||
    error.message.includes('Failed to fetch dynamically imported module') ||
    error.message.includes('dynamically imported module') ||
    error.message.includes('Importing a module script failed')
  );
}
