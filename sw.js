/* Quintet — service worker : version en ligne en priorité, copie locale si hors ligne */
const CACHE='quintet-v2';
const CORE=['./','./index.html','./manifest.webmanifest','./icons/icon-192.png','./icons/icon-512.png','./icons/maskable-512.png','./icons/apple-touch-icon.png','./icons/favicon-32.png'];
self.addEventListener('install',e=>{e.waitUntil(caches.open(CACHE).then(c=>c.addAll(CORE)).then(()=>self.skipWaiting()))});
self.addEventListener('activate',e=>{e.waitUntil(caches.keys().then(ks=>Promise.all(ks.filter(k=>k!==CACHE).map(k=>caches.delete(k)))).then(()=>self.clients.claim()))});
self.addEventListener('fetch',e=>{const r=e.request;if(r.method!=='GET')return;
  const u=new URL(r.url),ext=/fonts\.(googleapis|gstatic)\.com|cdnjs\.cloudflare\.com/.test(u.host);
  if(u.origin!==location.origin&&!ext)return;
  const isPage=r.mode==='navigate'||u.pathname.endsWith('/index.html')||u.pathname.endsWith('/');
  e.respondWith(caches.open(CACHE).then(c=>{
    const net=fetch(r,isPage?{cache:'no-cache'}:{}).then(res=>{if(res&&(res.ok||res.type==='opaque'))c.put(r,res.clone());return res});
    /* page du jeu : réseau d'abord (mise à jour immédiate), cache si hors ligne ; le reste : cache d'abord */
    if(isPage)return net.catch(()=>c.match(r,{ignoreSearch:true}).then(h=>h||c.match('./index.html')));
    return c.match(r).then(h=>h||net)}))});
