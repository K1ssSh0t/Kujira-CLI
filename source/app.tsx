import React, { useState, useEffect } from 'react';
import { Text, Box, Newline, useInput } from 'ink';
import SelectInput from 'ink-select-input';
import axios from 'axios';
import open from 'open';

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

export default function App({ provider }: Props) {
	const [selectedProvider, setSelectedProvider] = useState<string | null>(provider || null);
	const [animeList, setAnimeList] = useState<AnimeItem[]>([]);
	const [loading, setLoading] = useState(false);
	const [selectedAnime, setSelectedAnime] = useState<EpisodeDetails | null>(null);
	const [view, setView] = useState<'list' | 'episode' | 'info'>('list');
	const [animeInfo, setAnimeInfo] = useState<AnimeInfo | null>(null);

	// Add currentAnimeItem state to store the selected anime from the list
	const [currentAnimeItem, setCurrentAnimeItem] = useState<AnimeItem | null>(null);

	// Add new state for pagination
	const [currentPage, setCurrentPage] = useState(0);
	const ITEMS_PER_PAGE = 10;

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
			setView('episode');
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
			setView('info');
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

	const handleBack = () => {
		setView('list');
		setSelectedAnime(null);
	};

	const handleBackToProvider = () => {
		setSelectedProvider(null);
		setAnimeList([]);
	};

	const playVideo = async (server: Server) => {
		const mpvArgs = [
			'--force-window=immediate',
			`--http-header-fields=Referer: ${server.url}`,
		];

		if (server.source) {
			await open(server.source, {
				app: {
					name: 'mpv',
					arguments: mpvArgs,
				},
			});
		} else {
			await open(server.url, {
				app: {
					name: 'mpv',
					arguments: mpvArgs,
				},
			});
		}
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

	useInput((input, _key) => {
		if (input === 'q') {
			process.exit(0); // Exit the program
		}
		if (input === 'b') {
			if (view === 'info') {
				setView('episode');
				setAnimeInfo(null);
				setCurrentPage(0); // Reset pagination when going back
			} else if (view === 'episode') {
				handleBack();
			} else if (view === 'list') {
				handleBackToProvider();
			}
		}
		if (input === 'i' && view === 'episode' && currentAnimeItem) {
			fetchAnimeInfo(currentAnimeItem);
		}
		if (view === 'info') {
			if (input === 'n') handlePageChange('next');
			if (input === 'p') handlePageChange('prev');
		}
	});

	// Update info view
	if (view === 'info' && animeInfo) {
		const start = currentPage * ITEMS_PER_PAGE;
		const end = start + ITEMS_PER_PAGE;
		const currentEpisodes = animeInfo.episodes.slice(start, end);
		const totalPages = Math.ceil(animeInfo.episodes.length / ITEMS_PER_PAGE);

		return (
			<Box borderStyle="round" borderColor="blue" padding={1}>
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
		return (
			<Box borderStyle="round" borderColor="blue" padding={1}>
				<Box flexDirection="column">
					<Text bold>{selectedAnime.title}</Text>
					<Newline />
					<Text>Available Servers:</Text>
					<Newline />
					<SelectInput
						items={selectedAnime.servers.map((server) => ({
							label: `${server.name}: ${server.url}`,
							value: JSON.stringify(server)
						}))}
						onSelect={(item) => playVideo(JSON.parse(item.value))}
					/>
					<Newline />
					<Text color="yellow">
						Press 'i' for anime info | Press 'b' to go back to list | Press 'q' to quit
					</Text>
				</Box>
			</Box>
		);
	}

	if (!selectedProvider) {
		return (
			<Box borderStyle="round" borderColor="blue" padding={1}>
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
			<Box borderStyle="round" borderColor="blue" padding={1}>
				<Text>Loading...</Text>
			</Box>
		);
	}

	return (
		<Box borderStyle="round" borderColor="blue" padding={1}>
			<Box flexDirection="column">
				<Text>
					Latest from <Text color="green">{selectedProvider}</Text>:
				</Text>
				<Newline />
				<SelectInput
					items={animeList.map((anime) => ({
						label: `${anime.title} - Episode ${anime.episode}`,
						value: JSON.stringify(anime)
					}))}
					onSelect={(item) => handleAnimeSelect(JSON.parse(item.value))}
				/>
				<Newline />
				<Text color="yellow">Press 'b' to go back to provider selection | Press 'q' to quit</Text>
			</Box>
		</Box>
	);
}