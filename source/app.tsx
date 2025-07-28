import React, { useState, useEffect } from 'react';
import { Text, Box, Newline, useInput, useApp } from 'ink';
import SelectInput from 'ink-select-input';
import axios from 'axios';
import open from 'open';
import TextInput from 'ink-text-input';
import { getVoeMp4 } from './extractors/voe.js';
import { extractStreamTapeUrl } from './extractors/streamtape.js';

type AnimeItem = {
	title: string;
	url: string;
	img: string;
	episode: string;
};

type Server = {
	name: string;
	url: string;
	source: string;
};

type EpisodeDetails = {
	title: string;
	numberEpisode: string;
	servers: Server[];
};

type Props = {
	provider?: string;
};

// Update AnimeInfo type
type AnimeInfo = {
	title: string;
	title_alt: string;
	img: string;
	status: boolean;
	synopsis: string;
	genres: string[];
	episodes: {
		name: string;
		thumbnail: string;
		url: string;
		num: number;
	}[];
	related: {
		title: string;
		url: string;
		img: string;
	}[];
};

// Primero, añade un nuevo tipo para el historial de vistas
type ViewType = 'list' | 'episode' | 'info' | 'search';

export default function App({ provider }: Props) {
	const { exit } = useApp();
	const [selectedProvider, setSelectedProvider] = useState<string | null>(provider || null);
	const [animeList, setAnimeList] = useState<AnimeItem[]>([]);
	const [loading, setLoading] = useState(false);
	const [selectedAnime, setSelectedAnime] = useState<EpisodeDetails | null>(null);
	const [view, setView] = useState<ViewType>('list');
	const [animeInfo, setAnimeInfo] = useState<AnimeInfo | null>(null);

	// Add currentAnimeItem state to store the selected anime from the list
	const [currentAnimeItem, setCurrentAnimeItem] = useState<AnimeItem | null>(null);
	// Add near other state declarations
	const [isSearching, setIsSearching] = useState(false);
	// Add new state for pagination
	const [currentPage, setCurrentPage] = useState(0);
	const ITEMS_PER_PAGE = 10;

	// Add search state
	const [searchQuery, setSearchQuery] = useState('');
	const [searchResults, setSearchResults] = useState<Array<{
		name: string;
		image: string;
		url: string;
		type: string;
	}>>([]);

	// Añade el estado para el historial de vistas cerca de los otros estados
	const [viewHistory, setViewHistory] = useState<ViewType[]>(['list']);

	useEffect(() => {
		if (selectedProvider) {
			fetchAnimeList();
		}
	}, [selectedProvider]);

	const fetchAnimeList = async () => {
		setLoading(true);
		try {
			const response = await axios.get(
				`https://kujira.unusualoticramo.workers.dev/${selectedProvider}/latest`,
				{
					headers: {
						'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36',
						'Accept': 'application/json',
					}
				}
			);
			setAnimeList(response.data);
		} catch (error) {
			if (axios.isAxiosError(error)) {
				console.error(`Error: ${error.response?.status} - ${error.message}`);
			} else {
				console.error('An unexpected error occurred');
			}
		} finally {
			setLoading(false);
		}
	};

	const fetchEpisodeDetails = async (url: string) => {
		setLoading(true);
		try {
			const response = await axios.get(
				`https://kujira.unusualoticramo.workers.dev${url}`,
				{
					headers: {
						'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36',
						'Accept': 'application/json',
					}
				}
			);
			setSelectedAnime(response.data);
			pushView('episode');
		} catch (error) {
			if (axios.isAxiosError(error)) {
				console.error(`Error: ${error.response?.status} - ${error.message}`);
			} else {
				console.error('An unexpected error occurred');
			}
		} finally {
			setLoading(false);
		}
	};

	// Add fetch function for anime info
	const fetchAnimeInfo = async (animeItem: AnimeItem) => {
		setLoading(true);
		try {
			// Reset pagination before fetching new anime info
			setCurrentPage(0);

			// Remove episode number and 'episode' from URL
			const animeUrl = animeItem.url.replace(/\/episode\/(.+?)-\d+$/, '/anime/$1');

			const response = await axios.get(
				`https://kujira.unusualoticramo.workers.dev${animeUrl}`,
				{
					headers: {
						'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36',
						'Accept': 'application/json',
					}
				}
			);
			setAnimeInfo(response.data);
			pushView('info');
		} catch (error) {
			if (axios.isAxiosError(error)) {
				console.error(`Error: ${error.response?.status} - ${error.message}`);
			} else {
				console.error('An unexpected error occurred');
			}
		} finally {
			setLoading(false);
		}
	};

	const handleSelect = (item: { value: string }) => {
		setSelectedProvider(item.value);
	};

	const handleAnimeSelect = (anime: AnimeItem) => {
		setCurrentAnimeItem(anime);
		fetchEpisodeDetails(anime.url);
	};

	// const handleBack = () => {
	// 	setView('list');
	// 	setSelectedAnime(null);
	// 	resetSearchStates();
	// };

	const handleBackToProvider = () => {
		setSelectedProvider(null);
		setAnimeList([]);
	};

	const playVideo = async (server: Server) => {
		const mpvArgs = [
			'--force-window=immediate',
			`--http-header-fields=Referer: ${server.url}`,
		];

		let videoUrl = server.source;


		if (server.name.toLowerCase().includes('voe')) {

			videoUrl = await getVoeMp4(server.url) as string;
		}
		if (server.name == 'Stape') {

			videoUrl = await extractStreamTapeUrl(server.url);

		}

		if (!videoUrl) {
			throw new Error('Could not extract video URL');
		}

		await open(videoUrl, {
			app: {
				name: 'mpv',
				arguments: mpvArgs,
			},
		});
	};

	const items = [
		{ label: 'AnimeFLV', value: 'animeflv' },
		{ label: 'TioAnime', value: 'tioanime' },
	];

	// Add function to handle page navigation
	const handlePageChange = (direction: 'next' | 'prev') => {
		if (direction === 'next' && animeInfo) {
			const maxPages = Math.ceil(animeInfo.episodes.length / ITEMS_PER_PAGE) - 1;
			setCurrentPage(prev => Math.min(prev + 1, maxPages));
		} else {
			setCurrentPage(prev => Math.max(prev - 1, 0));
		}
	};


	// New search function using the selected provider
	const searchAnime = async (query: string) => {
		setLoading(true);
		try {
			const response = await axios.get(
				`https://kujira.unusualoticramo.workers.dev/${selectedProvider}/search?title=${encodeURIComponent(query)}`,
				{
					headers: {
						'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36',
						'Accept': 'application/json',
					}
				}
			);
			setSearchResults(response.data);
			pushView('search');
		} catch (error) {
			if (axios.isAxiosError(error)) {
				console.error(`Error: ${error.response?.status} - ${error.message}`);
			} else {
				console.error('An unexpected error occurred');
			}
		} finally {
			setLoading(false);
		}
	};

	// Primero, vamos a crear una función helper para resetear los estados de búsqueda
	const resetSearchStates = () => {
		setIsSearching(false);
		setSearchQuery('');
		setSearchResults([]);
	};

	// Modifica la función que cambia las vistas para mantener el historial
	const pushView = (newView: ViewType) => {
		setViewHistory(prev => [...prev, newView]);
		setView(newView);
	};

	// Añade una función para manejar el regreso
	const handleViewBack = () => {
		if (viewHistory.length > 1) {
			const newHistory = [...viewHistory];
			newHistory.pop(); // Elimina la vista actual
			const previousView = newHistory[newHistory.length - 1] || 'list';
			setView(previousView);
			setViewHistory(newHistory);
		}
	};

	useInput((input, key) => {
		if (input === 'q') {
			console.clear();
			exit();
		}

		if (isSearching) {
			if (key.escape) {
				resetSearchStates();
			}
			return;
		}

		if (input === 'b') {
			if (!isSearching && view === 'list') {
				handleBackToProvider();
			} else {
				handleViewBack();
				if (view === 'list') {
					resetSearchStates();
				}
			}
		}

		if (input === 'i' && view === 'episode' && currentAnimeItem) {
			fetchAnimeInfo(currentAnimeItem);
		}

		if (view === 'info') {
			if (input === 'n') handlePageChange('next');
			if (input === 'p') handlePageChange('prev');
		}

		// Solo activar la búsqueda si estamos en la vista de lista y no estamos ya buscando
		if (input === 's' && view === 'list' && !isSearching) {
			setIsSearching(true);
			setSearchQuery('');
		}
	});

	// // Add clear screen effect when changing views
	// useEffect(() => {
	// 	console.clear(); // Clear the terminal
	// }, [view, selectedProvider]); // Re-run when view or provider changes

	// Update info view
	if (view === 'info' && animeInfo) {
		const start = currentPage * ITEMS_PER_PAGE;
		const end = start + ITEMS_PER_PAGE;
		const currentEpisodes = animeInfo.episodes.slice(start, end);
		const totalPages = Math.ceil(animeInfo.episodes.length / ITEMS_PER_PAGE);

		return (
			<Box flexDirection="column" height="100%" borderStyle="round" borderColor="blue" padding={1}>
				<Box flexDirection="column">
					<Text bold>{animeInfo.title}</Text>
					{animeInfo.title_alt !== animeInfo.title && (
						<>
							<Text>Alt Title: {animeInfo.title_alt}</Text>
							<Newline />
						</>
					)}
					<Text>Status: {animeInfo.status ? 'Ongoing' : 'Finished'}</Text>
					<Newline />
					<Text>Genres: {animeInfo.genres.join(', ')}</Text>
					<Newline />
					<Text>Synopsis:</Text>
					<Text wrap="wrap">{animeInfo.synopsis}</Text>
					<Newline />
					<Text bold>Episodes:</Text>
					<Text color="blue">Page {currentPage + 1} of {totalPages}</Text>
					<Newline />
					<SelectInput
						items={currentEpisodes.map((episode) => ({
							label: `Episode ${episode.num}: ${episode.name}`,
							value: episode.url
						}))}
						onSelect={(item) => fetchEpisodeDetails(item.value)}
					/>
					<Newline />
					{currentPage > 0 && <Text>Press 'p' for previous page</Text>}
					{end < animeInfo.episodes.length && <Text>Press 'n' for next page</Text>}
					<Text color="yellow">Press 'b' to go back to servers | Press 'q' to quit</Text>
				</Box>
			</Box>
		);
	}

	// In episode view, add info instruction
	if (view === 'episode' && selectedAnime) {
		const allowedServers = ['stape', 'streamtape', 'voe', 'yourupload'];

		return (
			<Box flexDirection="column" height="100%" borderStyle="round" borderColor="blue" padding={1}>
				<Box flexDirection="column">
					<Text bold>{selectedAnime.title}</Text>
					<Newline />
					<Text>Available Servers:</Text>
					<Newline />
					<SelectInput
						items={selectedAnime.servers.map((server, index) => {
							const isAllowed = allowedServers.some(name =>
								server.name.toLowerCase().includes(name)
							);
							return {
								label: `${isAllowed ? '' : '[X] '}${server.name} : ${server.url}`,
								value: isAllowed ? JSON.stringify(server) : `disabled-${index}`,
								isDisabled: !isAllowed,

							};
						})}
						onSelect={(item) => {
							if (item.value && !item.value.startsWith('disabled-')) {
								playVideo(JSON.parse(item.value));
							}
						}}
					/>
					<Newline />
					<Text color="yellow">
						Press 'i' for anime info | Press 'b' to go back to list | Press 'q' to quit
					</Text>
				</Box>
			</Box>
		);
	}

	// Search results view
	if (view === 'search' && searchResults.length > 0) {
		return (
			<Box flexDirection="column" height="100%" borderStyle="round" borderColor="blue" padding={1}>
				<Box flexDirection="column">
					<Text>Search results for: <Text color="green">{searchQuery}</Text></Text>
					<Newline />
					<SelectInput
						items={searchResults.map((anime) => ({
							label: `${anime.name} (${anime.type})`,
							value: anime.url
						}))}
						onSelect={(item) => fetchAnimeInfo({ url: item.value } as AnimeItem)}
					/>
					<Newline />
					<Text color="yellow">Press 'b' to go back to list | Press 'q' to quit</Text>
				</Box>
			</Box>
		);
	} else if (view === 'search') {
		return (
			<Box flexDirection="column" height="100%" borderStyle="round" borderColor="blue" padding={1}>
				<Text>No results found for: {searchQuery}</Text>
				<Newline />
				<Text color="yellow">Press 'b' to go back to list | Press 'q' to quit</Text>
			</Box>
		);
	}

	if (!selectedProvider) {
		return (
			<Box flexDirection="column" height="100%" borderStyle="round" borderColor="blue" padding={1}>
				<Box flexDirection="column">
					<Text>Select anime provider:</Text>
					<SelectInput items={items} onSelect={handleSelect} />
					<Newline />
					<Text color="yellow">Press 'q' to quit</Text>
				</Box>
			</Box>
		);
	}

	if (loading) {
		return (
			<Box flexDirection="column" height="100%" borderStyle="round" borderColor="blue" padding={1}>
				<Text>Loading...</Text>
			</Box>
		);
	}

	// Replace the existing main return statement
	return (
		<Box flexDirection="column" height="100%" borderStyle="round" borderColor="blue" padding={1}>
			<Box flexDirection="column">
				<Text>
					Latest from <Text color="green">{selectedProvider}</Text>:
				</Text>
				<Newline />
				{isSearching ? (
					<>
						<Text>Search anime: </Text>
						<TextInput
							value={searchQuery}
							onChange={(value) => setSearchQuery(value)}
							onSubmit={(value) => {
								if (value.trim()) {
									searchAnime(value);
								}
								setIsSearching(false);
							}}
						/>
						<Newline />
						<Text color="yellow">
							Press Enter to search | Press 'b' to cancel
						</Text>
					</>
				) : (
					<>
						<SelectInput
							items={animeList.map((anime) => ({
								label: `${anime.title} - Episode ${anime.episode}`,
								value: JSON.stringify(anime)
							}))}
							onSelect={(item) => handleAnimeSelect(JSON.parse(item.value))}
						/>
						<Newline />
						<Text color="yellow">
							Press 's' to search | Press 'b' to go back to provider selection | Press 'q' to quit
						</Text>
					</>
				)}
			</Box>
		</Box>
	);
}