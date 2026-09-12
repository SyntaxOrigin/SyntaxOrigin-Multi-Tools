// Bağımlılıksız statik arayüzü src/ klasöründen dist/'e kopyalar.
// Wails, üretim ikilisinde frontend/dist içeriğini gömerek sunar.
import { cp, mkdir, rm } from 'node:fs/promises';

await rm('dist', { recursive: true, force: true });
await mkdir('dist', { recursive: true });
await cp('src', 'dist', { recursive: true });
console.log('frontend → dist kopyalandı');