export function MagicCircle() {
  return (
    <svg className="magic-circle" viewBox="0 0 120 120" xmlns="http://www.w3.org/2000/svg">
      <circle cx="60" cy="60" r="56" fill="none" stroke="rgba(180,140,255,0.55)" strokeWidth="1" />
      <circle cx="60" cy="60" r="48" fill="none" stroke="rgba(180,140,255,0.35)" strokeWidth="0.6" />
      <circle cx="60" cy="60" r="40" fill="none" stroke="rgba(232,192,96,0.45)" strokeWidth="1" strokeDasharray="4 6" />

      <polygon
        points="60,6 112,90 8,90"
        fill="none"
        stroke="rgba(180,140,255,0.45)"
        strokeWidth="0.8"
      />
      <polygon
        points="60,114 8,30 112,30"
        fill="none"
        stroke="rgba(232,192,96,0.4)"
        strokeWidth="0.8"
      />

      {[0, 60, 120, 180, 240, 300].map((deg, i) => {
        const rad = (deg * Math.PI) / 180;
        const x = 60 + 56 * Math.sin(rad);
        const y = 60 - 56 * Math.cos(rad);
        return (
          <circle key={i} cx={x} cy={y} r="2.5" fill="rgba(232,192,96,0.8)" />
        );
      })}

      <circle cx="60" cy="60" r="5" fill="none" stroke="rgba(180,140,255,0.6)" strokeWidth="1" />
      <circle cx="60" cy="60" r="2" fill="rgba(232,192,96,0.7)" />
    </svg>
  );
}
