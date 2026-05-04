import DaemonDownload from '../components/DaemonDownload';
import './Download.css';

const Download = () => {
  return (
    <div className="download-page" data-testid="download-page">
      <DaemonDownload />
    </div>
  );
};

export default Download;
