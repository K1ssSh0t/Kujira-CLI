import React, { useState, useEffect } from 'react';
import { Text, Box, Newline } from 'ink';
import SelectInput from 'ink-select-input';
import axios from 'axios';

type AnimeItem = {
	title: string;
	url: string;
	img: string;
	episode: string;
};

type Props = {
	provider?: string;
};

export default function App({ provider }: Props) {
	const [selectedProvider, setSelectedProvider] = useState<string | null>(provider || null);
	const [animeList, setAnimeList] = useState<AnimeItem[]>([]);
	const [loading, setLoading] = useState(false);

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

	const handleSelect = (item: { value: string }) => {
		setSelectedProvider(item.value);
	};

	const items = [
		{ label: 'AnimeFLV', value: 'animeflv' },
		{ label: 'TioAnime', value: 'tioanime' },
	];

	if (!selectedProvider) {
		return (
			<Box flexDirection="column">
				<Text>Select anime provider:</Text>
				<SelectInput items={items} onSelect={handleSelect} />
			</Box>
		);
	}

	if (loading) {
		return <Text>Loading...</Text>;
	}

	return (
		<Box flexDirection="column">
			<Text>
				Latest from <Text color="green">{selectedProvider}</Text>:
			</Text>
			<Newline />
			{animeList.map((anime) => (
				<Box key={anime.url}>
					<Text>
						{anime.title} - Episode {anime.episode}
					</Text>
				</Box>
			))}
		</Box>
	);
}