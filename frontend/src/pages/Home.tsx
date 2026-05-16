import { useNavigate } from 'react-router-dom';
import { useUser } from '@clerk/react';
import './Home.css';

interface FeatureCard {
  title: string;
  description: string;
  route: string;
  icon: string;
  testId: string;
}

const FEATURE_CARDS: FeatureCard[] = [
  {
    title: 'Match History',
    description: 'Review your recent matches, track win rates, and analyse performance trends.',
    route: '/match-history',
    icon: '⚔️',
    testId: 'home-feature-match-history',
  },
  {
    title: 'Draft',
    description: 'Pick up where you left off in your current draft or review past sessions.',
    route: '/draft',
    icon: '🃏',
    testId: 'home-feature-draft',
  },
  {
    title: 'Decks',
    description: 'Manage your decks, track streaks, and export for Arena or other platforms.',
    route: '/decks',
    icon: '📚',
    testId: 'home-feature-decks',
  },
  {
    title: 'Collection',
    description: 'Browse your card collection, filter by set and rarity, and see completion stats.',
    route: '/collection',
    icon: '🗂️',
    testId: 'home-feature-collection',
  },
];

export default function Home() {
  const navigate = useNavigate();
  const { user } = useUser();

  const displayName = user?.firstName || user?.username || 'Planeswalker';

  return (
    <div className="home-page" data-testid="home-page">
      <div className="home-header">
        <h1 className="home-title" data-testid="home-title">
          Welcome back, {displayName}
        </h1>
        <p className="home-subtitle">
          Your MTGA companion — track, analyse, and improve your game.
        </p>
      </div>

      <div className="home-features" data-testid="home-features">
        {FEATURE_CARDS.map((card) => (
          <button
            key={card.route}
            className="home-feature-card"
            data-testid={card.testId}
            onClick={() => navigate(card.route)}
          >
            <span className="home-feature-icon" aria-hidden="true">
              {card.icon}
            </span>
            <div className="home-feature-content">
              <h2 className="home-feature-title">{card.title}</h2>
              <p className="home-feature-description">{card.description}</p>
            </div>
            <span className="home-feature-arrow" aria-hidden="true">→</span>
          </button>
        ))}
      </div>
    </div>
  );
}
