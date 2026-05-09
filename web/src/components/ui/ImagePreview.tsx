import { useState } from "react";
import "./ImagePreview.css";

interface ImagePreviewProps {
  url: string;
  alt?: string;
  height?: number;
}

export function ImagePreview({ url, alt = "Preview", height = 180 }: ImagePreviewProps) {
  const [error, setError] = useState(false);

  if (!url || error) {
    return (
      <div className="img-preview img-preview--empty" style={{ height }}>
        <span className="img-preview__placeholder">No image</span>
      </div>
    );
  }

  return (
    <div className="img-preview" style={{ height }}>
      <img
        src={url}
        alt={alt}
        className="img-preview__img"
        onError={() => setError(true)}
        loading="lazy"
      />
    </div>
  );
}
