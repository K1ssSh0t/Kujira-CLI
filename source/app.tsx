import React, { useState, useEffect } from 'react';
import { Text, Box, Newline, useInput } from 'ink';
import SelectInput from 'ink-select-input';
import axios from 'axios';

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

export default function App({ provider }: Props) {
	const [selectedProvider, setSelectedProvider] = useState<string | null>(provider || null);
	const [animeList, setAnimeList] = useState<AnimeItem[]>([]);
	const [loading, setLoading] = useState(false);
	const [selectedAnime, setSelectedAnime] = useState<EpisodeDetails | null>(null);
	const [view, setView] = useState<'list' | 'episode'>('list');

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

	const handleSelect = (item: { value: string }) => {
		setSelectedProvider(item.value);
	};

	const handleAnimeSelect = (anime: AnimeItem) => {
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

	const items = [
		{ label: 'AnimeFLV', value: 'animeflv' },
		{ label: 'TioAnime', value: 'tioanime' },
	];

	useInput((input, key) => {
		if (input === 'q') {
			process.exit(0); // Exit the program
		}
		if (input === 'b') {
			if (view === 'episode') {
				handleBack();
			} else if (view === 'list') {
				handleBackToProvider();
			}
		}
	});

	if (!selectedProvider) {
		return (
			<Box flexDirection="column">
				<Text>Select anime provider:</Text>
				<SelectInput items={items} onSelect={handleSelect} />
				<Newline />
				<Text color="yellow">Press 'q' to quit</Text>
			</Box>
		);
	}

	if (loading) {
		return <Text>Loading...</Text>;
	}

	if (view === 'episode' && selectedAnime) {
		return (
			<Box flexDirection="column">
				<Text bold>{selectedAnime.title}</Text>
				<Newline />
				<Text>Available Servers:</Text>
				<Newline />
				{selectedAnime.servers.map((server) => (
					<Box key={server.name}>
						<Text>
							{server.name}: <Text color="blue">{server.url}</Text>
						</Text>
					</Box>
				))}
				<Newline />
				<Text color="yellow">Press 'b' to go back to list | Press 'q' to quit</Text>
			</Box>
		);
	}

	return (
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
	);
}