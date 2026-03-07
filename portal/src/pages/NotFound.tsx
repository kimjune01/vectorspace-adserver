import { Link } from 'react-router-dom';

export function NotFound() {
  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="text-center">
        <h1 className="text-4xl font-bold text-slate-300">404</h1>
        <p className="mt-2 text-slate-500">Page not found</p>
        <Link to="/admin" className="mt-4 inline-block text-blue-600 hover:underline">
          Go to Admin Dashboard
        </Link>
      </div>
    </div>
  );
}
