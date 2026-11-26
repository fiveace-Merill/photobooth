import fs from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const sharpMod = await import(pathToFileURL(path.resolve('web/node_modules/sharp/dist/index.mjs')).href);
const sharp = sharpMod.default;

const root = process.cwd();
const srcDir = path.join(root, 'web/public/images');
const outDir = path.join(root, 'web/public/demo');
const previewDir = path.join(outDir, 'previews');

const mapping = [
  ['aleksandra-sapozhnikova-uMyaAPXkeTg-unsplash.jpg', 'aleksandra-sapozhnikova.jpg'],
  ['bryan-heng-Aw1o_ObEDYk-unsplash.jpg', 'bryan-heng.jpg'],
  ['eren-yildiz-gxM_l567eBY-unsplash.jpg', 'eren-yildiz.jpg'],
  ['portraits-of-modern-dairy-CPdVPU2xJE0-unsplash.jpg', 'modern-dairy-portraits.jpg'],
  ['samuel-boyer--xs3dTqEhSM-unsplash.jpg', 'samuel-boyer.jpg'],
  ['sarmat-batagov-3OnNzf7eayk-unsplash.jpg', 'sarmat-batagov.jpg'],
  ['vaclav-pechar-BwhnE_QlXA8-unsplash.jpg', 'vaclav-pechar-lakeside.jpg'],
  ['vaclav-pechar-Ct7365AumXI-unsplash.jpg', 'vaclav-pechar-coast.jpg'],
];

await fs.mkdir(previewDir, { recursive: true });
const manifest = [];
for (const [srcName, destName] of mapping) {
  const src = path.join(srcDir, srcName);
  const dest = path.join(outDir, destName);
  const preview = path.join(previewDir, destName);
  await fs.copyFile(src, dest);
  const meta = await sharp(src).metadata();
  await sharp(src)
    .rotate()
    .resize({ width: 1200, height: 1200, fit: 'inside', withoutEnlargement: true })
    .jpeg({ quality: 70, mozjpeg: true })
    .toFile(preview);
  const previewMeta = await sharp(preview).metadata();
  manifest.push({
    src: srcName,
    file: destName,
    original: { width: meta.width, height: meta.height, bytes: (await fs.stat(dest)).size },
    preview: { width: previewMeta.width, height: previewMeta.height, bytes: (await fs.stat(preview)).size },
  });
}
await fs.writeFile(path.join(outDir, 'manifest.json'), JSON.stringify(manifest, null, 2));
console.log(JSON.stringify(manifest, null, 2));
