// Magic byte binary signatures
const MAGIC_BYTES: Record<string, string[]> = {
  pdf: ['25504446'], // %PDF
  zip: ['504B0304', '504B0506', '504B0708'], // PK..
  png: ['89504E47'],
  jpg: ['FFD8FF'],
};

export async function validateMagicBytes(file: File): Promise<boolean> {
  // Plain text / SRT / VTT files don't have binary headers; pass through
  const ext = file.name.split('.').pop()?.toLowerCase();
  if (['txt', 'srt', 'vtt'].includes(ext ?? '')) {
    return true;
  }

  const expectedSignatures = MAGIC_BYTES[ext ?? ''];
  if (!expectedSignatures) {
    return true; // fallback for unmapped plain files
  }

  // Read first 4 bytes of file
  const buffer = await file.slice(0, 4).arrayBuffer();
  const bytes = new Uint8Array(buffer);
  const hex = Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
    .toUpperCase();

  return expectedSignatures.some((sig) => hex.startsWith(sig));
}
