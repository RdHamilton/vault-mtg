import './CardHoverPreview.css';

export interface CardHoverPreviewProps {
  imageURL?: string;
  name: string;
  typeLine?: string;
  manaCost?: string;
  power?: string;
  toughness?: string;
  text?: string;
  setCode?: string;
  rarity?: string;
  // Recommendation-specific fields
  score?: number;
  confidence?: number;
  reasoning?: string;
  position: { x: number; y: number };
}

export default function CardHoverPreview({
  imageURL,
  name,
  typeLine,
  manaCost,
  power,
  toughness,
  text,
  setCode,
  rarity,
  score,
  confidence,
  reasoning,
  position,
}: CardHoverPreviewProps) {
  return (
    <div
      className="card-hover-preview"
      style={{
        position: 'fixed',
        left: Math.min(position.x, window.innerWidth - 280),
        top: Math.max(position.y, 10),
        zIndex: 10000,
      }}
    >
      <div className="preview-card">
        {imageURL && (
          <img
            src={imageURL}
            alt={name}
            className="preview-image"
          />
        )}
        <div className="preview-details">
          <h3 className="preview-name">{name}</h3>
          {typeLine && <p className="preview-type">{typeLine}</p>}
          <div className="preview-stats">
            {manaCost && (
              <span className="preview-mana">Mana: {manaCost}</span>
            )}
            {power && toughness && (
              <span className="preview-pt">{power}/{toughness}</span>
            )}
          </div>
          {text && <p className="preview-text">{text}</p>}

          {/* Score info for recommendations */}
          {score !== undefined && (
            <div className="preview-score-section">
              <span className="preview-score">
                Score: {Math.round(score * 100)}%
              </span>
              {confidence !== undefined && (
                <span className="preview-confidence">
                  Confidence: {Math.round(confidence * 100)}%
                </span>
              )}
            </div>
          )}
          {reasoning && (
            <p className="preview-reasoning">{reasoning}</p>
          )}

          {(setCode || rarity) && (
            <p className="preview-set">
              {setCode?.toUpperCase()}{setCode && rarity ? ' • ' : ''}{rarity}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
